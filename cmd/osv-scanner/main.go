package main

import (
	"fmt"
	"io"
	"os"

	"github.com/google/osv-scanner/internal/output"
	"github.com/google/osv-scanner/pkg/osvscanner"

	"github.com/urfave/cli/v2"
)

func run(args []string, stdout, stderr io.Writer) int {
	var r *output.Reporter

	app := &cli.App{
		Name:      "osv-scanner",
		Usage:     "scans various mediums for dependencies and matches it against the OSV database",
		Suggest:   true,
		Writer:    stdout,
		ErrWriter: stderr,
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:      "docker",
				Aliases:   []string{"D"},
				Usage:     "scan docker image with this name",
				TakesFile: false,
			},
			&cli.StringSliceFlag{
				Name:      "lockfile",
				Aliases:   []string{"L"},
				Usage:     "scan package lockfile on this path",
				TakesFile: true,
			},
			&cli.StringSliceFlag{
				Name:      "sbom",
				Aliases:   []string{"S"},
				Usage:     "scan sbom file on this path",
				TakesFile: true,
			},
			&cli.StringFlag{
				Name:      "config",
				Usage:     "set/override config file",
				TakesFile: true,
			},
			&cli.BoolFlag{
				Name:  "json",
				Usage: "sets output to json (WIP)",
			},
			&cli.BoolFlag{
				Name:  "skip-git",
				Usage: "skip scanning git repositories",
				Value: false,
			},
			&cli.BoolFlag{
				Name:    "recursive",
				Aliases: []string{"r"},
				Usage:   "check subdirectories",
				Value:   false,
			},
		},
		ArgsUsage: "[directory1 directory2...]",
		Action: func(context *cli.Context) error {
			r = output.NewReporter(stdout, stderr, context.Bool("json"))

			hydratedResp, query, err := osvscanner.DoScan(osvscanner.ScannerActions{
				LockfilePaths:        context.StringSlice("lockfile"),
				SBOMPaths:            context.StringSlice("sbom"),
				DockerContainerNames: context.StringSlice("docker"),
				Recursive:            context.Bool("recursive"),
				SkipGit:              context.Bool("skip-git"),
				ConfigOverridePath:   context.String("config"),
				DirectoryPaths:       context.Args().Slice(),
			}, r)

			if err != nil {
				return err
			}

			err = r.PrintResult(*query, hydratedResp)
			if err != nil {
				return fmt.Errorf("Failed to write output: %v", err)
			}

			return nil
		},
	}

	if err := app.Run(args); err != nil {
		r.PrintError(fmt.Sprintf("%v\n", err))
		return 1
	}

	return 0
}

func main() {
	os.Exit(run(os.Args, os.Stdout, os.Stderr))
}
