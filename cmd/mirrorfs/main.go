package main

import (
	"os"

	"github.com/joho/godotenv"
	"github.com/urfave/cli"

	fs "github.com/bbengfort/mirrorfs"
)

func main() {

	// Load the .env file if it exists
	godotenv.Load()

	// Instantiate the command line application
	app := cli.NewApp()
	app.Name = "mirrofs"
	app.Version = "0.1"
	app.Usage = "simple FUSE file system that mirrors a directory"

	// Define commands available to the application
	app.Commands = []cli.Command{
		{
			Name:     "mount",
			Usage:    "run the mirrorfs server on a directory",
			Category: "server",
			Action:   mount,
			Flags: []cli.Flag{
				cli.UintFlag{
					Name:   "verbosity",
					Usage:  "set log level from 0-4, lower is more verbose",
					Value:  2,
					EnvVar: "MIRRORFS_VERBOSITY",
				},
			},
		},
	}

	// Run the CLI program
	app.Run(os.Args)
}

func mount(c *cli.Context) (err error) {
	// Set the debug log level
	verbose := c.Uint("verbosity")
	fs.SetLogLevel(uint8(verbose))

	// Mount the directory with the arguments
	if c.NArg() != 2 {
		return cli.NewExitError("specify the mount and mirror directories", 1)
	}

	args := c.Args()
	if err := fs.Mount(args.Get(0), args.Get(1)); err != nil {
		return err
	}

	return nil
}
