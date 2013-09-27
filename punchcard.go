// Package punchtime provides cli access the web clock in Oracle Peoplesoft
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"

	"code.google.com/p/gopass"
	"github.com/PuerkitoBio/goquery"
	"github.com/tmc/keyring"
)

var (
	rootURL    = flag.String("rootURL", "", "Base URL to try to interact with this beast. Uses `PUNCHCARD_ROOT` if blank.")
	user       = flag.String("user", "", "Username, attempts to user env var `PUNCHCARD_USER` if blank")
	resavePass = flag.Bool("resave", false, "forces prompting for password")
	debug      = flag.Bool("debug", false, "show request details for debugging purposes")
	punchIn    = flag.Bool("in", false, "punch in")
	punchOut   = flag.Bool("out", false, "punch out")
)

var urls = map[string]string{
	"init":           "/psp/hrprd/?cmd=login",
	"login":          "/psp/hrprd/?cmd=login&languageCd=ENG",
	"timesheet":      "/psc/hrprd_1/EMPLOYEE/HRMS/c/ROLE_EMPLOYEE.TL_MSS_EE_SRCH_PRD.GBL",
	"timeclock_form": "/psc/hrprd/EMPLOYEE/HRMS/c/ROLE_EMPLOYEE.TL_SS_JOB_SRCH_CLK.GBL",
	"timeclock":      "/psc/hrprd/EMPLOYEE/HRMS/c/ROLE_EMPLOYEE.TL_WEBCLK_ESS.GBL",
}

func NewClient(BaseURL string) *PunchCardClient {
	cj, err := cookiejar.New(nil)
	if err != nil {
		panic(err)
	}
	return &PunchCardClient{
		BaseURL: BaseURL,
		Client:  &http.Client{Jar: cj},
	}
}

type PunchCardClient struct {
	BaseURL string
	*http.Client
}

func NewTimeSheet(timesheetMarkup io.Reader) (*Timesheet, error) {
	return parseTimesheet(timesheetMarkup)
}

// Init contacts the initial url to start the cookie session
func (p *PunchCardClient) Init() error {
	_, err := p.doGet(urls["init"])
	return err
}

func (p *PunchCardClient) doGet(url string) ([]byte, error) {
	r, err := p.Get(*rootURL + url)
	if err != nil {
		log.Fatal(err)
	}
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if *debug {
		fmt.Println(string(body))
	}
	return body, err
}

func (p *PunchCardClient) Login(username, password string) error {
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
	if *debug {
		fmt.Println(string(body))
	}
	return err
}

func (p *PunchCardClient) Timesheet() (*Timesheet, error) {
	buf, err := p.doGet(urls["timesheet"])

	if err != nil {
		return nil, err
	}
	return NewTimeSheet(bytes.NewReader(buf))
}

func (p *PunchCardClient) Clock(punchType PunchType) error {
	var doc *goquery.Document
	if r, err := p.doGet(urls["timeclock_form"]); err != nil {
		log.Fatal(err)
	} else {
		var docerr error
		doc, docerr = goquery.NewDocumentFromReader(bytes.NewReader(r))
		if err != nil {
			log.Fatal(docerr)
		}
	}

	var punchTypeStr string
	if punchType == IN {
		punchTypeStr = "1"
	} else if punchType == OUT {
		punchTypeStr = "2"
	} else {
		return fmt.Errorf("Unrecognized punch type:", punchType)
	}

	icsid, _ := doc.Find("#ICSID").Attr("value")
	icStateNum, _ := doc.Find("#ICStateNum").Attr("value")

	r, err := p.PostForm(p.BaseURL+urls["timeclock"], url.Values{
		"ICAJAX":                    {"1"},
		"ICAction":                  {"TL_LINK_WRK_TL_SAVE_PB$0"},
		"ICStateNum":                {icStateNum},
		"ICSID":                     {icsid},
		"TL_RPTD_TIME_PUNCH_TYPE$0": {punchTypeStr},
		"TASKGROUP$0":               {"PSNONCATSK"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if *debug {
		fmt.Println(string(body))
	}

	if err != nil {
		return err
	}
	return nil
}

func main() {
	flag.Parse()

	if *rootURL == "" {
		*rootURL = os.Getenv("PUNCHCARD_ROOT")
	}
	if *rootURL == "" {
		log.Fatal("no -rootURL supplied")
	}

	client := NewClient(*rootURL)

	err := client.Init()

	if err != nil {
		log.Fatal(err)
	}

	if *user == "" {
		*user = os.Getenv("PUNCHCARD_USER")
	}
	if *user == "" {
		fmt.Fprintln(os.Stderr, "No user supplied.")
		fmt.Print("enter username: ")
		n, err := fmt.Scanln(user)
		log.Println("scan:", n, err)
		log.Printf("'%s'\n", *user)
	}

	keyringKey := "punchcard" + *rootURL

	pw, err := keyring.Get(keyringKey, *user)

	if err == keyring.ErrNotFound || *resavePass {
		pw, err = gopass.GetPass("enter password: ")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		err = keyring.Set(keyringKey, *user, pw)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	if err := client.Login(*user, pw); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if ts, err := client.Timesheet(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	} else {
		fmt.Println("EmployeeID:", ts.EmployeeID)
		fmt.Println("Punch Status:", ts.Status())
		fmt.Println("This week:", ts.Punches.Duration())

		if *punchIn {
			if ts.Status() == IN {
				fmt.Fprintf(os.Stderr, "Refusing to punch in twice")
				os.Exit(1)
			}
			err := client.Clock(IN)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error punching in:", err)
				os.Exit(1)
			}
		} else if *punchOut {
			if ts.Status() != IN {
				fmt.Fprintf(os.Stderr, "Refusing to punch out twice")
				os.Exit(1)
			}
			err := client.Clock(OUT)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error punching out:", err)
				os.Exit(1)
			}
		}
	}
}
