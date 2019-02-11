package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/MITLibraries/fml"
	"github.com/davecgh/go-spew/spew"
)

// RetrieveRules for parsing MARC
func RetrieveRules(rulefile string) ([]*Rule, error) {
	// Open the file.
	file, err := os.Open(rulefile)
	if err != nil {
		return nil, err
	}

	// Schedule the file to be closed once
	// the function returns.
	defer file.Close()

	// Decode the file into a slice of pointers
	// to Feed values.
	var rules []*Rule
	err = json.NewDecoder(file).Decode(&rules)
	// We don't need to check for errors, the caller can do this.
	return rules, err
}

type marcparser struct {
	file          io.Reader
	rules         []*Rule
	languageCodes map[string]string
	countryCodes  map[string]string
}

//MarcGenerator parses binary MARC records.
type MarcGenerator struct {
	marcfile  io.Reader
	rulesfile string
}

//Generate a channel of Records.
func (m *MarcGenerator) Generate() <-chan Record {
	rules, err := RetrieveRules(m.rulesfile)
	if err != nil {
		spew.Dump(err)
	}

	languageCodes, err := RetrieveCodelist("language", "config/languages.xml")
	if err != nil {
		spew.Dump(err)
	}

	countryCodes, err := RetrieveCodelist("country", "config/countries.xml")
	if err != nil {
		spew.Dump(err)
	}

	out := make(chan Record)
	p := marcparser{file: m.marcfile, rules: rules, languageCodes: languageCodes,
		countryCodes: countryCodes}
	go p.parse(out)
	return out
}

func (m *marcparser) parse(out chan Record) {
	mr := fml.NewMarcIterator(m.file)

	for mr.Next() {
		record, err := mr.Value()

		if err != nil {
			log.Println("Error parsing MARC record:", record.ControlNum())
			log.Println(err)
			log.Println(record)
			continue
		}

		r, err := marcToRecord(record, m.rules, m.languageCodes, m.countryCodes)
		if err != nil {
			log.Println(err)
		} else {
			out <- r
		}
	}
	close(out)
}

func marcToRecord(fmlRecord fml.Record, rules []*Rule, languageCodes map[string]string, countryCodes map[string]string) (r Record, err error) {
	err = nil
	r = Record{}

	r.Identifier = fmlRecord.ControlNum()

	r.Source = "MIT Aleph"
	r.SourceLink = "https://library.mit.edu/item/" + r.Identifier
	r.OclcNumber = applyRule(fmlRecord, rules, "oclc_number")

	lccn := applyRule(fmlRecord, rules, "lccn")
	if lccn != nil {
		r.Lccn = strings.TrimSpace(lccn[0])
	}

	title := applyRule(fmlRecord, rules, "title")
	if title != nil {
		r.Title = title[0]
	} else {
		err = fmt.Errorf("Record %s has no title, check validity", r.Identifier)
		return r, err
	}

	r.AlternateTitles = applyRule(fmlRecord, rules, "alternate_titles")
	r.Creator = applyRule(fmlRecord, rules, "creators")
	r.Contributor = getContributors(fmlRecord, rules, "contributors")

	r.RelatedPlace = applyRule(fmlRecord, rules, "related_place")
	r.RelatedItems = getRelatedItems(fmlRecord, rules, "related_items")

	r.InBibliography = applyRule(fmlRecord, rules, "in_bibliography")

	r.Subject = applyRule(fmlRecord, rules, "subjects")

	r.Isbn = applyRule(fmlRecord, rules, "isbns")
	r.Issn = applyRule(fmlRecord, rules, "issns")
	r.Doi = applyRule(fmlRecord, rules, "dois")

	country := applyRule(fmlRecord, rules, "place_of_publication")
	if country != nil {
		country[0] = strings.Trim(country[0], " |")
		r.Country = TranslateCodes(country, countryCodes)[0]
	}

	// TODO: use lookup tables to translate returned codes to values
	r.Language = applyRule(fmlRecord, rules, "languages")
	r.Language = TranslateCodes(r.Language, languageCodes)

	r.CallNumber = applyRule(fmlRecord, rules, "call_numbers")

	edition := applyRule(fmlRecord, rules, "edition")
	if edition != nil {
		r.Edition = edition[0]
	}

	r.Imprint = applyRule(fmlRecord, rules, "imprint")

	description := applyRule(fmlRecord, rules, "physical_description")
	if description != nil {
		r.PhysicalDescription = description[0]
	}

	r.PublicationFrequency = applyRule(fmlRecord, rules, "publication_frequency")

	// publication year
	date := applyRule(fmlRecord, rules, "publication_date")
	if date != nil {
		r.PublicationDate = date[0]
	}

	numbering := applyRule(fmlRecord, rules, "numbering")
	if numbering != nil {
		r.Numbering = numbering[0]
	}

	r.Notes = applyRule(fmlRecord, rules, "notes")

	r.Contents = applyRule(fmlRecord, rules, "contents")

	r.Summary = applyRule(fmlRecord, rules, "summary")

	r.ContentType = contentType(fmlRecord.Leader.Type)

	lf := applyRule(fmlRecord, rules, "literary_form")
	r.LiteraryForm = literaryForm(lf)

	r.Links = getLinks(fmlRecord)
	r.Holdings = getHoldings(fmlRecord, "866", []string{"b", "c", "h", "a", "z"})

	if len(r.Holdings) == 0 {
		r.Holdings = getHoldings(fmlRecord, "852", []string{"b", "c", "h", "a", "z", "k"})
	}

	for _, h := range r.Holdings {
		if !stringInSlice(h.Format, r.Format) {
			r.Format = append(r.Format, h.Format)
		}
	}

	return r, err
}

func applyRule(fmlRecord fml.Record, rules []*Rule, field string) []string {
	recordFieldRule := getRules(rules, field)

	res := extractData(recordFieldRule, fmlRecord)
	return res
}

// takes a supplied marc rule and fmlRecord returns an array of stringified subfields
func extractData(rule *Rule, fmlRecord fml.Record) []string {
	var field []string
	for _, r := range rule.Fields {
		f := filter(fmlRecord, r)
		for _, y := range f {
			if !stringInSlice(y, field) {
				field = append(field, y)
			}
		}
	}
	return field
}

// stringInSlice determines whether a supplied string is an item in a supplied slice.
// Returns true if the string is in the slice, and returns false otherwise.
// Taken from https://stackoverflow.com/a/15323988
func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func filter(fmlRecord fml.Record, field *Field) []string {
	var stuff []string
	values := fmlRecord.Filter(field.Tag + field.Subfields)
	for _, f := range values {
		v := strings.Join(f, " ")
		if field.Bytes != "" {
			f := strings.Split(field.Bytes, ":")
			first, _ := strconv.Atoi(f[0])
			take, _ := strconv.Atoi(f[1])
			v = v[first:(first + take)]
		}
		stuff = append(stuff, v)
	}
	return stuff
}

// returns slice of contributors of marc fields taking into account the rules for which fields and subfields we care about as defined in marc_rules.json
func getContributors(fmlRecord fml.Record, rules []*Rule, field string) []*Contributor {
	recordFieldRule := getRules(rules, field)
	var c []*Contributor

	for _, r := range recordFieldRule.Fields {
		y := new(Contributor)
		y.Kind = r.Kind

		y.Value = filter(fmlRecord, r)

		if y.Value != nil {
			c = append(c, y)
		}
	}

	return c
}

// returns slice of related items of marc fields taking into account the rules for which fields and subfields we care about as defined in marc_rules.json
func getRelatedItems(fmlRecord fml.Record, rules []*Rule, field string) []*RelatedItem {
	recordFieldRule := getRules(rules, field)
	var c []*RelatedItem
	for _, r := range recordFieldRule.Fields {
		y := new(RelatedItem)
		y.Kind = r.Kind
		y.Value = filter(fmlRecord, r)
		if y.Value != nil {
			c = append(c, y)
		}
	}
	return c
}

// returns all rules that match a supplied fieldname
func getRules(rules []*Rule, label string) *Rule {
	for _, v := range rules {
		if v.Label == label {
			return v
		}
	}
	return nil // TODO: this will lead to a panic and end the world. While this is ultimately an appropriate response to failing to find rules we expect to find, it would be better to handle that explictly and log something that explains it before terminating cleanly.
}

func literaryForm(x []string) string {
	var t string
	if x == nil {
		return ""
	}
	switch x[0] {
	case "0", "s", "e":
		t = "nonfiction"
	default:
		t = "fiction"
	}
	return t
}

// Content type mappings
func contentType(x byte) string {
	var t string
	switch x {
	case 'c':
		t = "Musical score"
	case 'd':
		t = "Musical score"
	case 'e':
		t = "Cartographic material"
	case 'f':
		t = "Cartographic material"
	case 'g':
		t = "Moving image"
	case 'i':
		t = "Sound recording"
	case 'j':
		t = "Sound recording"
	case 'k':
		t = "Still image"
	case 'm':
		t = "Computer file"
	case 'o':
		t = "Kit"
	case 'p':
		t = "Mixed materials"
	case 'r':
		t = "Object"
	default:
		t = "Text"
	}
	return t
}

// RetrieveCodelist retrieves language codes for parsing MARC languages
func RetrieveCodelist(codeType string, filePath string) (map[string]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatal(err)
	}
	// Language struct
	type CodeMap struct {
		Name string `xml:"name"`
		Code string `xml:"code"`
	}

	decoder := xml.NewDecoder(file)
	codes := make(map[string]string)

	for {
		t, _ := decoder.Token()
		if t == nil {
			break
		}
		switch se := t.(type) {
		case xml.StartElement:
			if se.Name.Local == codeType {
				var c CodeMap
				decoder.DecodeElement(&c, &se)
				codes[c.Code] = c.Name
			}
		}
	}
	return codes, err
}

// TranslateCodes takes an array of MARC language/country codes and returns the language/country names.
func TranslateCodes(recordCodes []string, codeMap map[string]string) []string {
	var names []string
	for _, c := range recordCodes {
		name := codeMap[c]
		if name != "" {
			names = append(names, name)
		} else {
			names = append(names, c)
		}
	}
	return names
}

// getLinks take a MARC record and eturns an array of Link objects from the 856 field data.
func getLinks(fmlRecord fml.Record) []Link {
	var links []Link
	marc856 := fmlRecord.DataField("856")
	if len(marc856) == 0 {
		return nil
	}
	for _, f := range marc856 {
		ind1 := string(f.Indicator1)
		ind2 := string(f.Indicator2)

		if ind1 == "4" && (ind2 == "0" || ind2 == "1") {
			link := Link{
				Kind:         subfieldValue(f.SubFields, "3"),
				URL:          subfieldValue(f.SubFields, "u"),
				Text:         subfieldValue(f.SubFields, "y"),
				Restrictions: subfieldValue(f.SubFields, "z")}
			if link.Kind == "" {
				link.Kind = "unknown"
			}
			links = append(links, link)
		}
	}
	return links
}

// getLocations takes a MARC record and returns an array of Holdings objects.
// The expecation is to use either an 852 or an 866 field.
func getHoldings(fmlRecord fml.Record, tag string, subfieldCodes []string) []Holding {
	var holdings []Holding
	df := fmlRecord.DataField(tag)
	if len(df) == 0 {
		return nil
	}
	for _, f := range df {
		holding := Holding{
			Location: lookupLocation(subfieldValue(f.SubFields, subfieldCodes[0])),
			Collection: lookupCollection(
				subfieldValue(f.SubFields, subfieldCodes[1]),
				subfieldValue(f.SubFields, subfieldCodes[0])),
			CallNumber: subfieldValue(f.SubFields, subfieldCodes[2]),
			Summary:    subfieldValue(f.SubFields, subfieldCodes[3]),
			Notes:      subfieldValue(f.SubFields, subfieldCodes[4])}
		if tag == "866" {
			holding.Format = "Print volume"
		} else {
			holding.Format = lookupFormat(holding.Location, subfieldValue(f.SubFields, subfieldCodes[5]))
		}
		holdings = append(holdings, holding)
	}
	return holdings
}

func subfieldValue(subs []fml.SubField, code string) string {
	for _, x := range subs {
		if x.Code == code {
			return x.Value
		}
	}
	return ""
}

func lookupCollection(col string, loc string) string {
	var c string
	switch col {
	case "STACK":
		c = "Stacks"
	case "ATLCS":
		c = "Atlas Case"
	case "AUDBK":
		c = "Audiobooks"
	case "JRNAL":
		if loc == "HUM" {
			c = "Humanities Journals"
		} else if loc == "SCI" {
			c = "Science Journals"
		} else {
			c = "Journal Collection"
		}
	case "BRWS":
		c = "Browsery"
	case "CNSUS":
		c = "Census Collection"
	case "CIRCD":
		c = "Service Desk"
	case "DETEC":
		c = "Detective Fiction Collection"
	case "EJ":
		c = "Electronic Journal"
	case "GIS":
		c = "GIS Collection"
	case "GOV":
		c = "Government Documents"
	case "GRNVL":
		c = "Graphic Novel Collection"
	case "HDCBX":
		c = "Harvard Depository Boxed Items"
	case "ICPSR":
		c = "ICPSR Codebooks"
	case "IMPLS":
		c = "Impulse Borrowing Display"
	case "LSA4":
		c = "Journal Collection"
	case "OVRSZ":
		c = "Oversize Materials"
	case "LMTED":
		c = "Limited Access Collection"
	case "MAPRM":
		c = "Map Room"
	case "MFORM":
		c = "Microforms"
	case "MEDIA":
		c = "Media"
	case "NCIP":
		c = "BLC ILB Item"
	case "NEWBK":
		c = "Science New Books Display"
	case "NOLN1":
		c = "Noncirculating Collection 1"
	case "NOLN2":
		c = "Noncirculating Collection 2"
	case "NOLN3":
		c = "Noncirculating Collection 3"
	case "OCC":
		c = "Off Campus Collection"
	case "OCCBX":
		c = "Off Campus Collection Boxed Items"
	case "OFFCT":
		c = "Offsite Cataloging"
	case "PAMPH":
		c = "Pamphlet Collection"
	case "PRECT":
		if loc == "HUM" {
			c = "Humanities Pre-cataloged Collection"
		} else if loc == "SCI" {
			c = "Science Pre-cataloged Collection"
		} else {
			c = "Pre-cataloged Collection"
		}
	case "REF":
		c = "Reference Collection"
	case "RSERV":
		c = "Reserve Stacks"
	case "SWING":
		c = "Basement Grammar Books"
	case "TRAVL":
		c = "Travel Collection"
	case "UNCAT":
		c = "Uncataloged Materials - see Librarian"
	case "UNKNW":
		c = "Problems Materials - see Librarian"
	case "WSTM":
		c = "Women in Science, Technology, and Medicine"
	default:
		c = col
	}

	return c
}

func lookupLocation(loc string) string {
	var t string
	switch loc {
	case "HUM", "RBR", "SCI":
		t = "Hayden Library"
	case "MIT50":
		t = "MIT Administrative Library"
	case "ARC":
		t = "Institute Archives"
	case "ACQ":
		t = "Institute Archives"
	case "ENG":
		t = "Barker Library"
	case "CAT":
		t = "Cataloging and Metadata Services"
	case "DEW":
		t = "Dewey Library"
	case "DIR":
		t = "Director's Office"
	case "DOC":
		t = "Document Services"
	case "ILB":
		t = "Interlibrary Borrowing"
	case "LSA":
		t = "Library Storage Annex"
	case "NET":
		t = "Internet Resource"
	case "MUS":
		t = "Lewis Music Library"
	case "PHY":
		t = "Physics Department Reading Room"
	case "RTC":
		t = "Rotch Library"
	case "RVC":
		t = "Rotch Visual Collections"
	case "SPC":
		t = "Space Cntr: Ask library staff"
	case "OFFIC":
		t = "Office delivery"
	default:
		t = loc
	}
	return t
}

func lookupFormat(loc string, format_code string) string {
	var t string
	if loc != "Internet Resource" {
		switch format_code {
		case "BOOKS", "REGULAR":
			t = "Print volume"
		case "ATLAS":
			t = "Atlas"
		case "AUDIO", "AUDTAPE":
			t = "Audio tape"
		case "CD":
			t = "Compact disc"
		case "CDROM":
			t = "CD-ROM"
		case "DSKETTE":
			t = "Diskette"
		case "DVD":
			t = "DVD-ROM"
		case "FICHE":
			t = "Microfiche"
		case "FOLIO", "OVRSIZE":
			t = "Oversized print volume"
		case "MAP":
			t = "Map sheet"
		case "MFILM":
			t = "Microfilm"
		case "RECORD":
			t = "Audio record"
		case "SCORE":
			t = "Musical score"
		case "SMALL":
			t = "Undersized print volume"
		case "VDISC":
			t = "Videodisc"
		case "VHS":
			t = "VHS"
		default:
			t = "Print volume"
		}
	}
	return t
}
