package main

// Record struct stores our internal mappings of data and is used to when
// mapping various external data sources before sending to elasticsearch
type Record struct {
	Identifier           string         `json:"identifier"`
	Title                string         `json:"title"`
	AlternateTitles      []string       `json:"alternate_titles,omitempty"`
	Creator              []string       `json:"creators,omitempty"`
	Contributor          []*Contributor `json:"contributors,omitempty"`
	Subject              []string       `json:"subjects,omitempty"`
	Isbn                 []string       `json:"isbns,omitempty"`
	Issn                 []string       `json:"issns,omitempty"`
	Doi                  []string       `json:"dois,omitempty"`
	OclcNumber           []string       `json:"oclcs,omitempty"`
	Lccn                 string         `json:"lccn,omitempty"`
	Country              string         `json:"country_of_publication,omitempty"`
	Language             []string       `json:"languages,omitempty"`
	PublicationDate      string         `json:"publication_date,omitempty"`
	ContentType          string         `json:"content_type,omitempty"`
	CallNumber           []string       `json:"call_numbers,omitempty"`
	Edition              string         `json:"edition,omitempty"`
	Imprint              []string       `json:"imprint,omitempty"`
	PhysicalDescription  string         `json:"physical_description,omitempty"`
	PublicationFrequency []string       `json:"publication_frequency,omitempty"`
	Numbering            string         `json:"numbering,omitempty"`
	Notes                []string       `json:"notes,omitempty"`
	Contents             []string       `json:"contents,omitempty"`
	Summary              []string       `json:"summary,omitempty"`
	Format               []string       `json:"format,omitempty"`
	LiteraryForm         string         `json:"literary_form,omitempty"`
	RelatedPlace         []string       `json:"related_place,omitempty"`
	InBibliography       []string       `json:"in_bibliography,omitempty"`
	RelatedItems         []*RelatedItem `json:"related_items,omitempty"`
	Links                []Link         `json:"links,omitempty"`
	Holdings             []Holdings     `json:"holdings,omitempty"`
}

// Contributor is a port of a Record
type Contributor struct {
	Kind  string   `json:"kind"`
	Value []string `json:"value"`
}

// RelatedItem is a port of a Record
type RelatedItem struct {
	Kind  string   `json:"kind"`
	Value []string `json:"value"`
}

// Link is a port of a Record
type Link struct {
	Kind         string `json:"kind"`
	Text         string `json:"text"`
	URL          string `json:"url"`
	Restrictions string `json:"restrictions"`
}

// Holdings is a port of a Record
type Holdings struct {
	Location   string `json:"location"`
	CallNumber string `json:"call_number"`
	Status     string `json:"status"`
}

// Rule defines where the rules are in JSON
type Rule struct {
	Label  string   `json:"label"`
	Array  bool     `json:"array"`
	Fields []*Field `json:"fields"`
}

// Field defines where the Fields within a Rule are in JSON
type Field struct {
	Tag       string `json:"tag"`
	Subfields string `json:"subfields"`
	Bytes     string `json:"bytes"`
	Kind      string `json:"kind"`
}

// Parser defines an interface common to parsers
type Parser interface {
	Parse(chan Record)
}

// Processor is an interface that allows converting from custom data into
// our Record structure
type Processor interface {
	Process()
}

var ingested int
