package main

import (
	"fmt"
	"github.com/mitlibraries/mario/pkg/client"
	"github.com/mitlibraries/mario/pkg/ingester"
	"github.com/urfave/cli"
	"log"
	"os"
)

func main() {
	var debug bool
	var auto bool
	var url, index string
	var v4 bool

	app := cli.NewApp()

	//Global options
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "url, u",
			Value:       "http://127.0.0.1:9200",
			Usage:       "URL for the Elasticsearch cluster",
			Destination: &url,
		},
		cli.StringFlag{
			Name:        "index, i",
			Usage:       "Name of the Elasticsearch index",
			Destination: &index,
		},
		cli.BoolFlag{
			Name:        "v4",
			Usage:       "Use AWS v4 signing",
			Destination: &v4,
		},
	}

	app.Commands = []cli.Command{
		{
			Name:      "ingest",
			Usage:     "Parse and ingest the input file",
			ArgsUsage: "[filepath, use format 's3://bucketname/objectname' for s3]",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "rules",
					Value: "/config/marc_rules.json",
					Usage: "Path to marc rules file",
				},
				cli.StringFlag{
					Name:  "consumer, c",
					Value: "es",
					Usage: "Consumer to use (es, json or title)",
				},
				cli.StringFlag{
					Name:  "type, t",
					Value: "marc",
					Usage: "Type of file to process",
				},
				cli.StringFlag{
					Name:  "prefix, p",
					Value: "aleph",
					Usage: "Index prefix to use: default is aleph",
				},
				cli.BoolFlag{
					Name:        "debug",
					Usage:       "Output debugging information",
					Destination: &debug,
				},
				cli.BoolFlag{
					Name:        "auto",
					Usage:       "Automatically promote / demote on completion",
					Destination: &auto,
				},
			},
			Action: func(c *cli.Context) error {
				var es *client.ESClient
				config := ingester.Config{
					Filename:  c.Args().Get(0),
					Consumer:  c.String("consumer"),
					Source:    c.String("type"),
					Index:     index,
					Prefix:    c.String("prefix"),
					Promote:   auto,
					Rulesfile: c.String("rules"),
				}
				stream, err := ingester.NewStream(config.Filename)
				if err != nil {
					return err
				}
				defer stream.Close()
				if config.Consumer == "es" {
					es, err = client.NewESClient(url, v4)
					if err != nil {
						return err
					}
				}

				ingest := ingester.Ingester{Stream: stream, Client: es}
				err = ingest.Configure(config)
				if err != nil {
					return err
				}
				count, err := ingest.Ingest()
				if debug {
					fmt.Printf("Total records ingested: %d\n", count)
				}
				return err
			},
		},
		{
			Name:  "indexes",
			Usage: "List Elasticsearch indexes",
			Action: func(c *cli.Context) error {
				es, err := client.NewESClient(url, v4)
				if err != nil {
					return err
				}
				indexes, err := es.Indexes()
				if err != nil {
					return err
				}
				for _, i := range indexes {
					fmt.Printf(`
Name: %s
  Documents: %d
  Health: %s
  Status: %s
  UUID: %s
  Size: %s
`, i.Index, i.DocsCount, i.Health, i.Status, i.UUID, i.StoreSize)
				}
				return nil
			},
		},
		{
			Name:  "aliases",
			Usage: "List Elasticsearch aliases and associated indexes",
			Action: func(c *cli.Context) error {
				es, err := client.NewESClient(url, v4)
				if err != nil {
					return err
				}
				aliases, err := es.Aliases()
				if err != nil {
					return err
				}
				for _, a := range aliases {
					fmt.Printf(`
Alias: %s
  Index: %s
`, a.Alias, a.Index)
				}
				return nil
			},
		},
		{
			Name:  "ping",
			Usage: "Ping Elasticsearch",
			Action: func(c *cli.Context) error {
				es, err := client.NewESClient(url, v4)
				if err != nil {
					return err
				}
				res, err := es.Ping(url)
				if err != nil {
					return err
				}
				fmt.Printf(`
Name: %s
Cluster: %s
Version: %s
Lucene version: %s
`, res.Name, res.ClusterName, res.Version.Number, res.Version.LuceneVersion)
				return nil
			},
		},
		{
			Name:     "delete",
			Usage:    "Delete an Elasticsearch index",
			Category: "Index actions",
			Action: func(c *cli.Context) error {
				es, err := client.NewESClient(url, v4)
				if err != nil {
					return err
				}
				err = es.Delete(index)
				return err
			},
		},
		{
			Name:     "promote",
			Usage:    "Promote Elasticsearch alias to prod",
			Category: "Index actions",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "prefix, p",
					Value: "aleph",
					Usage: "Index prefix to use: default is aleph",
				},
			},
			Action: func(c *cli.Context) error {
				es, err := client.NewESClient(url, v4)
				if err != nil {
					return err
				}
				err = es.Promote(index, c.String("prefix"))
				return err
			},
		},
		{
			Name:      "reindex",
			Usage:     "Reindex one index to another index.",
			UsageText: "Use the Elasticsearch reindex API to copy one index to another. The doc source must be present in the original index.",
			Category:  "Index actions",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "destination",
					Usage: "Name of new index",
				},
			},
			Action: func(c *cli.Context) error {
				es, err := client.NewESClient(url, v4)
				if err != nil {
					return err
				}
				count, err := es.Reindex(index, c.String("destination"))
				fmt.Printf("%d documents reindexed\n", count)
				return err
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
