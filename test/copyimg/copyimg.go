package main

import (
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/containers/image/copy"
	"github.com/containers/image/signature"
	"github.com/containers/image/storage"
	"github.com/containers/image/transports"
	"github.com/containers/image/types"
	"github.com/containers/storage/pkg/reexec"
	sstorage "github.com/containers/storage/storage"
	"github.com/urfave/cli"
)

func main() {
	if reexec.Init() {
		return
	}

	app := cli.NewApp()
	app.Name = "copyimg"
	app.Usage = "barebones image copier"
	app.Version = "0.0.1"

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug",
			Usage: "turn on debug logging",
		},
		cli.StringFlag{
			Name:  "root",
			Usage: "graph root directory",
		},
		cli.StringFlag{
			Name:  "runroot",
			Usage: "run root directory",
		},
		cli.StringFlag{
			Name:  "storage-driver",
			Usage: "storage driver",
		},
		cli.StringSliceFlag{
			Name:  "storage-option",
			Usage: "storage option",
		},
		cli.StringFlag{
			Name:  "signature-policy",
			Usage: "signature policy",
		},
		cli.StringFlag{
			Name:  "image-name",
			Usage: "set image name",
		},
		cli.StringFlag{
			Name:  "add-name",
			Usage: "name to add to image",
		},
		cli.StringFlag{
			Name:  "import-from",
			Usage: "import source",
		},
		cli.StringFlag{
			Name:  "export-to",
			Usage: "export target",
		},
	}

	app.Action = func(c *cli.Context) error {
		var store sstorage.Store
		var ref, importRef, exportRef types.ImageReference
		var err error

		debug := c.GlobalBool("debug")
		rootDir := c.GlobalString("root")
		runrootDir := c.GlobalString("runroot")
		storageDriver := c.GlobalString("storage-driver")
		storageOptions := c.GlobalStringSlice("storage-option")
		signaturePolicy := c.GlobalString("signature-policy")
		imageName := c.GlobalString("image-name")
		addName := c.GlobalString("add-name")
		importFrom := c.GlobalString("import-from")
		exportTo := c.GlobalString("export-to")

		if debug {
			logrus.SetLevel(logrus.DebugLevel)
		} else {
			logrus.SetLevel(logrus.ErrorLevel)
		}

		if imageName != "" {
			if rootDir == "" && runrootDir != "" {
				logrus.Errorf("must set --root and --runroot, or neither")
				os.Exit(1)
			}
			if rootDir != "" && runrootDir == "" {
				logrus.Errorf("must set --root and --runroot, or neither")
				os.Exit(1)
			}
			storeOptions := sstorage.DefaultStoreOptions
			if rootDir != "" && runrootDir != "" {
				storeOptions.GraphDriverName = storageDriver
				storeOptions.GraphDriverOptions = storageOptions
				storeOptions.GraphRoot = rootDir
				storeOptions.RunRoot = runrootDir
			}
			store, err = sstorage.GetStore(storeOptions)
			if err != nil {
				logrus.Errorf("error opening storage: %v", err)
				os.Exit(1)
			}
			defer store.Shutdown(false)

			storage.Transport.SetStore(store)
			ref, err = storage.Transport.ParseStoreReference(store, imageName)
			if err != nil {
				logrus.Errorf("error parsing image name: %v", err)
				os.Exit(1)
			}
		}

		systemContext := types.SystemContext{
			SignaturePolicyPath: signaturePolicy,
		}
		policy, err := signature.DefaultPolicy(&systemContext)
		if err != nil {
			logrus.Errorf("error loading signature policy: %v", err)
			os.Exit(1)
		}
		policyContext, err := signature.NewPolicyContext(policy)
		if err != nil {
			logrus.Errorf("error loading signature policy: %v", err)
			os.Exit(1)
		}
		defer policyContext.Destroy()
		options := &copy.Options{}

		if importFrom != "" {
			importRef, err = transports.ParseImageName(importFrom)
			if err != nil {
				logrus.Errorf("error parsing image name %v: %v", importFrom, err)
				os.Exit(1)
			}
		}

		if exportTo != "" {
			exportRef, err = transports.ParseImageName(exportTo)
			if err != nil {
				logrus.Errorf("error parsing image name %v: %v", exportTo, err)
				os.Exit(1)
			}
		}

		if imageName != "" {
			if importFrom != "" {
				err = copy.Image(policyContext, ref, importRef, options)
				if err != nil {
					logrus.Errorf("error importing %s: %v", importFrom, err)
					os.Exit(1)
				}
			}
			if addName != "" {
				destImage, err := storage.Transport.GetStoreImage(store, ref)
				if err != nil {
					logrus.Errorf("error finding image: %v", err)
					os.Exit(1)
				}
				names := append(destImage.Names, imageName, addName)
				err = store.SetNames(destImage.ID, names)
				if err != nil {
					logrus.Errorf("error adding name to %s: %v", imageName, err)
					os.Exit(1)
				}
			}
			if exportTo != "" {
				err = copy.Image(policyContext, exportRef, ref, options)
				if err != nil {
					logrus.Errorf("error exporting %s: %v", exportTo, err)
					os.Exit(1)
				}
			}
		} else {
			if importFrom != "" && exportTo != "" {
				err = copy.Image(policyContext, exportRef, importRef, options)
				if err != nil {
					logrus.Errorf("error copying %s to %s: %v", importFrom, exportTo, err)
					os.Exit(1)
				}
			}
		}
		return nil
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}
