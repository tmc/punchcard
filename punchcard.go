// Package punchtime provides cli access to web-based time punching.
//
// Initial target: Oracle Peoplesoft
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
)

var rootURL = flag.String("rootURL", "", "Base URL to try to interact with this beast")

var urls = map[string]string{
	"init":  "/psp/hrprd/?cmd=login",
	"login": "/psp/hrprd/?cmd=login&languageCd=ENG",
}

//name="login"
//<input type="hidden" name="timezoneOffset" value="0">
//
//<label class="onlineid">Online ID:</label>
//
//<!-- EDIT THIS LINE FOR THE PROPER INPUT NAME -->
//<input type="text" id="userid" name="userid" class="onlineid"/ tabindex="2"><br />
//
//                            <label class="password">Password</label>
//<!-- EDIT THIS LINE FOR THE PROPER INPUT NAME -->
//<input type="password" id="pwd" name="pwd" class="password" tabindex="3"/><br />
//
//                                <p class="forgotpasswordline"><a href="https://myidentity.ku.edu/password/forgot">Forgot your KU password?</a> <span class="divider">|</span> <a href="https://kumc-id.kumc.edu/IDM/jsps/pwdmgt/ForgotPassword.jsf">Forgot your KUMC password?</a></p>
//                                <input type="submit" name="submit" value="Sign in" class="signinbutton" onclick="submitAction(document.login)" tabindex="4"/>
//                                </form>

func NewClient(BaseURL string) *PunchTime {
	cj, err := cookiejar.New(nil)
	if err != nil {
		panic(err)
	}
	return &PunchTime{
		BaseURL: BaseURL,
		Client:  &http.Client{Jar: cj},
	}
}

type PunchTime struct {
	BaseURL string
	*http.Client
}

// Init contacts the initial url to start the cookie session
func (p *PunchTime) Init() error {
	return p.doGet(urls["init"])
}

func (p *PunchTime) doGet(url string) error {
	r, err := p.Get(*rootURL + url)
	if err != nil {
		log.Fatal(err)
	}
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	fmt.Println(string(body))
	return err
}

func (p *PunchTime) Login(username, password string) error {
	r, err := p.PostForm(p.BaseURL+urls["login"],
		url.Values{
			"pwd":            {password},
			"userid":         {username},
			"timezoneOffset": {fmt.Sprint(utcOffset().Minutes())},
			"submit":         {"Sign in"},
		})
	if err != nil {
		log.Fatal(err)
	}
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	fmt.Println(string(body))
	return err
}

func main() {
	flag.Parse()

	if *rootURL == "" {
		log.Fatal("no -rootURL supplied")
	}

	client := NewClient(*rootURL)

	err := client.Init()

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(utcOffset().Minutes())
}
