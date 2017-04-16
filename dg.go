package main

import (
	"bytes"
	"io/ioutil"
	"os/exec"
	"fmt"
	"net/url"
	"os"
	"strings"
	"path"
	"net/http"
	"runtime"
	"math/rand"
	"encoding/json"

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

	githubAuthHost = "localhost:4865"
	githubAuthRedirect = "/github"
	githubAuthClientID = "724d69696d6f403a31ea"
	githubAuthClientSecret = "SECRET_PLS_DONT_READ_IT"
	githubAuthRespError = `<html>
  <body>
    <h1>Error. <code>State</code> parameter isn't correct.</h1>
  </body>
</html>
`
	githubAuthRespSuccess = `<html>
  <body>
    <h1>Success. <small>Go back to your terminal :)</small></h1>
    <script>setTimeout(function(){window.close()}, 1000)</script>
  </body>
</html>
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
		{
			Name: "gh",
			Subcommands: []cli.Command{
				{
					Name: "clone-all",
					Usage: "clone all repositories from a GitHub user or org",
					Action: ghCloneAll,
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

func openBrowser(url string) (err error) {
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform: " + runtime.GOOS)
	}
	return
}

func randString(n int) string {
	const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func runWebServer(state string, csuccess chan<- string, cerror chan<- error) {
	// start a local web server
	http.HandleFunc(githubAuthRedirect, func(w http.ResponseWriter, req *http.Request) {
		queryString := req.URL.Query()
		responseCode := queryString.Get("code")
		responseState := queryString.Get("state")
		fmt.Printf("code: %s, state: %s\n", responseCode, responseState)
		if state == responseState {
			fmt.Fprintf(w, githubAuthRespSuccess)
			csuccess <- responseCode
			return
		} else {
			fmt.Fprintf(w, githubAuthRespError)
			err := fmt.Errorf("github oauth handler: state parameter doesn't match: expected=%s, actual=%s",
				state, responseState)
			cerror <- err
			return
		}
	})
	err := http.ListenAndServe(githubAuthHost, nil)
	if err != nil {
		cerror <- err
	}
}

type GithubOAuthToken struct {
	AccessToken string `json:"access_token"`
	TokenType string `json:"token_type"`
	Scope string `json:"scope"`
}

func ghCloneAll(_ *cli.Context) error {
	// OAuth authentication, quick & dirty:
	// 1. start a local web server
	// 2. open github OAuth authorize page in browser
	// 3. redirect to localhost:PORT
	// 4. stop the local web server
	// 5. use the state and code parameters to ask for an access token

	// channels
	chanError := make(chan error, 1)
	chanSuccess := make(chan string, 1)

	// start web server
	state := randString(16)
	go runWebServer(state, chanSuccess, chanError)

	// open browser and start OAuth protocol
	redirectURI := "http://" + githubAuthHost + githubAuthRedirect
	err := openBrowser("https://github.com/login/oauth/authorize?"+
		"client_id="+githubAuthClientID+
		"&redirect_uri="+redirectURI+
		"&scope=repo"+
		"&state="+state)
	if err != nil {
		return err
	}

	// handle result
	// TODO: stop and clean the web server
	var accessCode string
	select {
	case err = <-chanError: return err
	case accessCode = <-chanSuccess:
		fmt.Println("accessCode: "+accessCode)
	}

	// exchange code for a token
	params := url.Values{
		"client_id": {githubAuthClientID},
		"client_secret": {githubAuthClientSecret},
		"code": {accessCode},
		"state": {state},
	}
	client := &http.Client{}
	req, err := http.NewRequest("POST", "https://github.com/login/oauth/access_token", strings.NewReader(params.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("github oauth token exchange: error %d: %s", resp.StatusCode, body)
	}

	var token GithubOAuthToken
	json.Unmarshal(body, &token)
	fmt.Println("access_token:", token.AccessToken)

	// TODO: store the token in a safe way

	// TODO: query public and private repos using the token

	return nil
}
