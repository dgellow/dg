package main

import (
	"bytes"
	"os/exec"
	"fmt"
	"os"
	"strings"
	"path"

	"gopkg.in/urfave/cli.v1"
)

const (
	templateGoMain = `package main

import (
	"fmt"
)

func main() {
	fmt.Println("🤖 \tHello there. I just want to say that I ❤ you \n\t**bot-gigglies**")
}
`

	templateREADME = `# :REPLACEME:
`
	templateGoGitignore = `:REPLACEME:
`
)

func main() {
	app := cli.NewApp()
	app.Name = "dg"
	app.Version = "0.0.1"
	app.Commands = []cli.Command{
		{
			Name: "go",
			Subcommands: []cli.Command{
				{
					Name: "init",
					Usage: "initialize a new Go project",
					Action: goInit,
				},
			},
		},
	}

	app.Run(os.Args)
}

func createFile(path string, content string) error {
	fmt.Println("create: "+path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return err
	}
	_, err = f.WriteString(content)
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return nil
}

func goInit(_ *cli.Context) error {
	// main.go
	err := createFile("main.go", templateGoMain)
	if err != nil {
		return err
	}
	// README.md
	workingDir, err := os.Getwd()
	workingDir = path.Base(workingDir)
	if err != nil {
		return err
	}
	err = createFile("README.md", strings.Replace(templateREADME, ":REPLACEME:", workingDir, -1))
	if err != nil {
		return err
	}
	// .gitignore
	err = createFile(".gitignore", strings.Replace(templateGoGitignore, ":REPLACEME:", workingDir, -1))
	if err != nil {
		return err
	}
	// git init
	fmt.Println("run: git init")
	cmd := exec.Command("git", "init")
	stderr := bytes.Buffer{}
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		_, err = fmt.Fprint(os.Stderr, stderr.String())
		if err != nil {
			panic(err)
		}
		return err
	}
	// dep init
	fmt.Println("run: dep init")
	cmd = exec.Command("dep", "init")
	stderr = bytes.Buffer{}
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		_, err = fmt.Fprint(os.Stderr, stderr.String())
		if err != nil {
			panic(err)
		}
		return err
	}
	// dep ensure
	fmt.Println("run: dep ensure")
	cmd = exec.Command("dep", "ensure")
	stderr = bytes.Buffer{}
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		_, err = fmt.Fprint(os.Stderr, stderr.String())
		if err != nil {
			panic(err)
		}
		return err
	}
	return nil
}
