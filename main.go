package main // import "moul.io/ascii2svg"

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	cli "gopkg.in/urfave/cli.v2"
	"moul.io/ascii2svg/ascii2svg"
)

func main() {
	defaults := ascii2svg.DefaultOptions()
	app := cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "line-color",
				Usage: "color of the direction-connection layer (named color or #rgb hex)",
				Value: defaults.LineColor,
			},
			&cli.Float64Flag{
				Name:  "line-width",
				Usage: "stroke width of the direction-connection layer",
				Value: defaults.LineWidth,
			},
			&cli.Float64Flag{
				Name:  "cell-width",
				Usage: "monospace glyph cell width used by the direction layer",
				Value: defaults.CellWidth,
			},
			&cli.Float64Flag{
				Name:  "cell-height",
				Usage: "monospace glyph cell height used by the direction layer",
				Value: defaults.CellHeight,
			},
			&cli.Float64Flag{
				Name:  "offset-x",
				Usage: "x offset of the direction layer",
				Value: defaults.OffsetX,
			},
			&cli.Float64Flag{
				Name:  "offset-y",
				Usage: "y offset of the direction layer",
				Value: defaults.OffsetY,
			},
			&cli.IntFlag{
				Name:  "max-rows",
				Usage: "maximum number of input rows accepted by the direction extractor",
				Value: defaults.MaxRows,
			},
			&cli.IntFlag{
				Name:  "max-cols",
				Usage: "maximum number of visual columns accepted by the direction extractor",
				Value: defaults.MaxCols,
			},
			&cli.IntFlag{
				Name:  "max-cells",
				Usage: "maximum number of grid cells accepted by the direction extractor",
				Value: defaults.MaxCells,
			},
		},
		Action: run,
	}
	if err := app.Run(os.Args); err != nil {
		log.Printf("error: %v\n", err)
		os.Exit(1)
	}
}

func run(c *cli.Context) error {
	input := strings.Join(c.Args().Slice(), " ")
	if input == "" {
		inBytes, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		input = string(inBytes)
	}
	opts := ascii2svg.Options{
		LineColor:  c.String("line-color"),
		LineWidth:  c.Float64("line-width"),
		CellWidth:  c.Float64("cell-width"),
		CellHeight: c.Float64("cell-height"),
		OffsetX:    c.Float64("offset-x"),
		OffsetY:    c.Float64("offset-y"),
		MaxRows:    c.Int("max-rows"),
		MaxCols:    c.Int("max-cols"),
		MaxCells:   c.Int("max-cells"),
	}
	out, err := ascii2svg.FromStringWithOptions(input, opts)
	if err != nil {
		return err
	}
	fmt.Println(out)
	return nil
}
