package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/HouzuoGuo/laitos/bridge"
	"github.com/HouzuoGuo/laitos/feature"
	"github.com/HouzuoGuo/laitos/frontend/dnsd"
	"github.com/HouzuoGuo/laitos/frontend/httpd"
	"github.com/HouzuoGuo/laitos/frontend/smtpd"
	"github.com/HouzuoGuo/laitos/httpclient"
	"io/ioutil"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// Pretty much copied from other test cases.
func TestConfig(t *testing.T) {
	js := `{
  "Features": {
    "Shell": {
      "InterpreterPath": "/bin/bash"
    }
  },
  "Mailer": {
    "MailFrom": "howard@localhost",
    "MTAHost": "127.0.0.1",
    "MTAPort": 25
  },
  "DNSDaemon": {
    "ListenAddress": "127.0.0.1",
    "ListenPort": 61211,
    "ForwardTo": "8.8.8.8",
    "AllowQueryIPPrefixes": [
      "127.0"
    ],
    "PerIPLimit": 10
  },
  "HTTPDaemon": {
    "ListenAddress": "127.0.0.1",
    "ListenPort": 23486,
    "BaseRateLimit": 10,
    "ServeDirectories": {
      "/my/dir": "/tmp/test-laitos-dir2"
    }
  },
  "HealthCheck": {
    "TCPPorts": [
      9114
    ],
    "IntervalSec": 300,
    "Recipients": [
      "howard@localhost"
    ]
  },
  "HTTPBridges": {
    "TranslateSequences": {
      "Sequences": [
        [
          "alpha",
          "beta"
        ]
      ]
    },
    "PINAndShortcuts": {
      "PIN": "httpsecret",
      "Shortcuts": {
        "httpshortcut": ".secho httpshortcut"
      }
    },
    "NotifyViaEmail": {
      "Recipients": [
        "howard@localhost"
      ]
    },
    "LintText": {
      "TrimSpaces": true,
      "CompressToSingleLine": true,
      "KeepVisible7BitCharOnly": true,
      "CompressSpaces": true,
      "MaxLength": 35
    }
  },
  "HTTPHandlers": {
    "SelfTestEndpoint": "/test",
    "InformationEndpoint": "/info",
    "CommandFormEndpoint": "/cmd_form",
    "GitlabBrowserEndpoint": "/gitlab",
    "GitlabBrowserEndpointConfig": {
      "PrivateToken": "just a dummy token"
    },
    "IndexEndpoints": [
      "/",
      "/index.html"
    ],
    "IndexEndpointConfig": {
      "HTMLFilePath": "/tmp/test-laitos-index2.html"
    },
    "MailMeEndpoint": "/mail_me",
    "MailMeEndpointConfig": {
      "Recipients": [
        "howard@localhost"
      ]
    },
    "WebProxyEndpoint": "/proxy",
    "TwilioSMSEndpoint": "/sms",
    "TwilioCallEndpoint": "/call",
    "TwilioCallEndpointConfig": {
      "CallGreeting": "Hi there"
    }
  },
  "MailDaemon": {
    "ListenAddress": "127.0.0.1",
    "ListenPort": 18573,
    "PerIPLimit": 10,
    "ForwardTo": [
      "howard@localhost",
      "root@localhost"
    ]
  },
  "MailProcessor": {
    "CommandTimeoutSec": 10
  },
  "MailProcessorBridges": {
    "TranslateSequences": {
      "Sequences": [
        [
          "aaa",
          "bbb"
        ]
      ]
    },
    "PINAndShortcuts": {
      "PIN": "mailsecret",
      "Shortcuts": {
        "mailshortcut": ".secho mailshortcut"
      }
    },
    "NotifyViaEmail": {
      "Recipients": [
        "howard@localhost"
      ]
    },
    "LintText": {
      "TrimSpaces": true,
      "CompressToSingleLine": true,
      "MaxLength": 70
    }
  },
  "SockDaemon": {
    "ListenAddress": "127.0.0.1",
    "ListenPort": 6891,
    "PerIPLimit": 10,
    "Password": "1234567"
  },
  "TelegramBot": {
    "AuthorizationToken": "intentionally-bad-token"
  },
  "TelegramBotBridges": {
    "TranslateSequences": {
      "Sequences": [
        [
          "123",
          "456"
        ]
      ]
    },
    "PINAndShortcuts": {
      "PIN": "telegramsecret",
      "Shortcuts": {
        "telegramshortcut": ".secho telegramshortcut"
      }
    },
    "NotifyViaEmail": {
      "Recipients": [
        "howard@localhost"
      ]
    },
    "LintText": {
      "TrimSpaces": true,
      "CompressToSingleLine": true,
      "MaxLength": 120
    }
  }
}
`
	var config Config
	if err := config.DeserialiseFromJSON([]byte(js)); err != nil {
		t.Fatal(err)
	}

	// ============ Test DNS daemon ============
	// Update ad-server blacklist
	dnsDaemon := config.GetDNSD()
	if numEntries, err := dnsDaemon.InstallAdBlacklist(); err != nil || numEntries < 100 {
		t.Fatal(err, numEntries)
	}
	// Server should start within two seconds
	go func() {
		if err := dnsDaemon.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
	}()
	time.Sleep(2 * time.Second)

	serverAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:61211")
	if err != nil {
		t.Fatal(err)
	}
	githubComQuery, err := hex.DecodeString("97eb010000010000000000000667697468756203636f6d0000010001")
	if err != nil {
		t.Fatal(err)
	}
	packetBuf := make([]byte, dnsd.MaxPacketSize)
	clientConn, err := net.DialUDP("udp", nil, serverAddr)
	// Try to reach rate limit
	delete(dnsDaemon.BlackList, "github.com")
	var success int
	for i := 0; i < 100; i++ {
		if _, err := clientConn.Write(githubComQuery); err != nil {
			t.Fatal(err)
		}
		clientConn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		length, err := clientConn.Read(packetBuf)
		if err == nil && length > 50 {
			success++
		}
	}
	if success < 5 || success > 15 {
		t.Fatal(success)
	}
	// Wait out rate limit
	time.Sleep(dnsd.RateLimitIntervalSec * time.Second)
	// Blacklist github and see if query gets a black hole response
	dnsDaemon.BlackList["github.com"] = struct{}{}
	if _, err := clientConn.Write(githubComQuery); err != nil {
		t.Fatal(err)
	}
	clientConn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	respLen, err := clientConn.Read(packetBuf)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Index(packetBuf[:respLen], dnsd.BlackHoleAnswer) == -1 {
		t.Fatal("did not answer black hole")
	}

	// ============ Test health check ===========
	// Port is now listening
	go func() {
		listener, err := net.Listen("tcp", "127.0.0.1:9114")
		if err != nil {
			t.Fatal(err)
		}
		for {
			if _, err := listener.Accept(); err != nil {
				t.Fatal(err)
			}
		}
	}()
	time.Sleep(1 * time.Second)
	check := config.GetHealthCheck()
	if !check.Execute() {
		t.Fatal("some check failed")
	}
	// Break a feature
	check.Features.LookupByTrigger[".s"] = &feature.Shell{}
	if check.Execute() {
		t.Fatal("did not fail")
	}
	check.Features.LookupByTrigger[".s"] = &feature.Shell{InterpreterPath: "/bin/bash"}
	// Expect checks to begin within a second
	if err := check.Initialise(); err != nil {
		t.Fatal(err)
	}
	go func() {
		if err := check.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
	}()
	time.Sleep(1 * time.Second)

	// ============ Test HTTP daemon ============
	// (Essentially combine all cases of api_test.go and httpd_test.go)
	// Create a temporary file for index
	indexFile := "/tmp/test-laitos-index2.html"
	defer os.Remove(indexFile)
	if err := ioutil.WriteFile(indexFile, []byte("this is index #LAITOS_CLIENTADDR #LAITOS_3339TIME"), 0644); err != nil {
		t.Fatal(err)
	}
	// Create a temporary directory of file
	htmlDir := "/tmp/test-laitos-dir2"
	if err := os.MkdirAll(htmlDir, 0755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(htmlDir)
	if err := ioutil.WriteFile(htmlDir+"/a.html", []byte("a html"), 0644); err != nil {
		t.Fatal(err)
	}

	httpDaemon := config.GetHTTPD()

	if len(httpDaemon.SpecialHandlers) != 11 {
		// 1 x self test, 1 x sms, 2 x call, 1 x gitlab, 1 x mail me, 1 x proxy, 2 x index, 1 x cmd form, 1 x info
		t.Fatal(httpDaemon.SpecialHandlers)
	}
	// Find the randomly generated endpoint name for twilio call callback
	var twilioCallbackEndpoint string
	for endpoint := range httpDaemon.SpecialHandlers {
		switch endpoint {
		case "/sms":
		case "/call":
		case "/test":
		case "/cmd_form":
		case "/mail_me":
		case "/proxy":
		case "/info":
		case "/gitlab":
		case "/":
		case "/index.html":
		default:
			twilioCallbackEndpoint = endpoint
		}
	}
	t.Log("Twilio callback endpoint is located at", twilioCallbackEndpoint)
	go func() {
		if err := httpDaemon.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
	}()
	addr := "http://127.0.0.1:23486"
	time.Sleep(2 * time.Second)

	// Index handle
	for _, location := range []string{"/", "/index.html"} {
		resp, err := httpclient.DoHTTP(httpclient.Request{}, addr+location)
		expected := "this is index 127.0.0.1 " + time.Now().Format(time.RFC3339)
		if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != expected {
			t.Fatal(err, string(resp.Body), resp)
		}
	}
	// Directory handle
	resp, err := httpclient.DoHTTP(httpclient.Request{}, addr+"/my/dir")
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != `<pre>
<a href="a.html">a.html</a>
</pre>
` {
		t.Fatal(err, string(resp.Body), resp)
	}
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/my/dir/a.html")
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != "a html" {
		t.Fatal(err, string(resp.Body), resp)
	}
	// Non-existent paths
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/my/dir/doesnotexist.html")
	if err != nil || resp.StatusCode != http.StatusNotFound {
		t.Fatal(err, string(resp.Body), resp)
	}
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/doesnotexist")
	if err != nil || resp.StatusCode != http.StatusNotFound {
		t.Fatal(err, string(resp.Body), resp)
	}
	// Test hitting rate limits
	time.Sleep(httpd.RateLimitIntervalSec * time.Second)
	success = 0
	for i := 0; i < 200; i++ {
		resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/")
		expected := "this is index 127.0.0.1 " + time.Now().Format(time.RFC3339)
		if err == nil && resp.StatusCode == http.StatusOK && string(resp.Body) == expected {
			success++
		}
	}
	if success < 50 || success > 150 {
		t.Fatal(success)
	}
	// Wait till rate limits reset
	time.Sleep(httpd.RateLimitIntervalSec * time.Second)
	// Feature self test
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/test")
	if err != nil {
		t.Fatal(err, string(resp.Body), resp)
	}
	// If feature self test fails, the failure would only occur in contacting mailer
	mailFailure := ".m: dial tcp 127.0.0.1:25: getsockopt: connection refused<br/>\n"
	if resp.StatusCode == http.StatusInternalServerError && string(resp.Body) != mailFailure {
		t.Fatal(err, string(resp.Body), resp)
	}
	// System information
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/info")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "Public IP:") {
		t.Fatal(err, string(resp.Body))
	}
	// Gitlab handle
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/gitlab")
	if err != nil || resp.StatusCode != http.StatusOK || strings.Index(string(resp.Body), "Enter path to browse") == -1 {
		t.Fatal(err, string(resp.Body), resp)
	}
	// Command Form
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/cmd_form")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "submit") {
		t.Fatal(err, string(resp.Body))
	}
	resp, err = httpclient.DoHTTP(httpclient.Request{Method: http.MethodPost}, addr+"/cmd_form")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "submit") {
		t.Fatal(err, string(resp.Body))
	}
	resp, err = httpclient.DoHTTP(httpclient.Request{
		Method: http.MethodPost,
		Body:   strings.NewReader(url.Values{"cmd": {"httpsecret.sls /"}}.Encode()),
	}, addr+"/cmd_form")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "bin") {
		t.Fatal(err, string(resp.Body))
	}
	// MailMe
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/mail_me")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "submit") {
		t.Fatal(err, string(resp.Body))
	}
	resp, err = httpclient.DoHTTP(httpclient.Request{Method: http.MethodPost}, addr+"/mail_me")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.Contains(string(resp.Body), "submit") {
		t.Fatal(err, string(resp.Body))
	}
	resp, err = httpclient.DoHTTP(httpclient.Request{
		Method: http.MethodPost,
		Body:   strings.NewReader(url.Values{"msg": {"又给你发了一个邮件"}}.Encode()),
	}, addr+"/mail_me")
	if err != nil || resp.StatusCode != http.StatusOK ||
		(!strings.Contains(string(resp.Body), "发不出去") && !strings.Contains(string(resp.Body), "发出去了")) {
		t.Fatal(err, string(resp.Body))
	}
	// Web proxy
	// Normally the proxy should inject javascript into the page, but the home page does not look like HTML so proxy won't do that.
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/proxy?u=http%%3A%%2F%%2F127.0.0.1%%3A23486%%2F")
	if err != nil || resp.StatusCode != http.StatusOK || !strings.HasPrefix(string(resp.Body), "this is index") {
		t.Fatal(err, string(resp.Body))
	}
	// Twilio - exchange SMS with bad PIN
	resp, err = httpclient.DoHTTP(httpclient.Request{
		Method: http.MethodPost,
		Body:   strings.NewReader(url.Values{"Body": {"pin mismatch"}}.Encode()),
	}, addr+"/sms")
	if err != nil || resp.StatusCode != http.StatusNotFound {
		t.Fatal(err, resp)
	}
	// Twilio - exchange SMS, the extra spaces around prefix and PIN do not matter.
	resp, err = httpclient.DoHTTP(httpclient.Request{
		Method: http.MethodPost,
		Body:   strings.NewReader(url.Values{"Body": {"httpsecret .s echo 0123456789012345678901234567890123456789"}}.Encode()),
	}, addr+"/sms")
	expected := `<?xml version="1.0" encoding="UTF-8"?>
<Response><Message>01234567890123456789012345678901234</Message></Response>
`
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != expected {
		t.Fatal(err, resp)
	}
	// Twilio - check phone call greeting
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/call")
	expected = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
    <Gather action="%s" method="POST" timeout="30" finishOnKey="#" numDigits="1000">
        <Say>Hi there</Say>
    </Gather>
</Response>
`, twilioCallbackEndpoint)
	if err != nil || string(resp.Body) != expected {
		t.Fatalf("%+v\n%s\n%s", err, string(resp.Body), expected)
	}
	// Twilio - check phone call response to DTMF
	resp, err = httpclient.DoHTTP(httpclient.Request{
		Method: http.MethodPost,
		Body:   strings.NewReader(url.Values{"Digits": {"0000000"}}.Encode()),
	}, addr+twilioCallbackEndpoint)
	expected = `<?xml version="1.0" encoding="UTF-8"?>
<Response>
	<Say>Sorry</Say>
	<Hangup/>
</Response>
`
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != expected {
		t.Fatal(err, string(resp.Body))
	}
	// Twilio - check phone call response to command
	resp, err = httpclient.DoHTTP(httpclient.Request{
		Method: http.MethodPost,
		//                                             h t tp s   e c  r  e t .   s    tr  u e
		Body: strings.NewReader(url.Values{"Digits": {"4480870777733222777338014207777087778833"}}.Encode()),
	}, addr+twilioCallbackEndpoint)
	expected = fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Response>
    <Gather action="%s" method="POST" timeout="30" finishOnKey="#" numDigits="1000">
        <Say>EMPTY OUTPUT, repeat again, EMPTY OUTPUT, repeat again, EMPTY OUTPUT, over.</Say>
    </Gather>
</Response>
`, twilioCallbackEndpoint)
	if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != expected {
		t.Fatal(err, string(resp.Body))
	}

	// ============ Test mail processor ============
	mailproc := config.GetMailProcessor()
	pinMismatch := `From howard@localhost Sun Feb 26 18:17:34 2017
Return-Path: <howard@localhost>
X-Original-To: howard@localhost
Delivered-To: howard@localhost
Received: by localhost (Postfix, from userid 1000)
        id 542EA2421BD; Sun, 26 Feb 2017 18:17:34 +0100 (CET)
Date: Sun, 26 Feb 2017 18:17:34 +0100
To: howard@localhost
Subject: hi howard
User-Agent: Heirloom mailx 12.5 7/5/10
MIME-Version: 1.0
Content-Type: text/plain; charset=us-ascii
Content-Transfer-Encoding: 7bit
Message-Id: <20170226171734.542EA2421BD@localhost.>
From: howard@localhost (Howard Guo)
Status: R

PIN mismatch`
	if err := mailproc.Process([]byte(pinMismatch)); err != bridge.ErrPINAndShortcutNotFound {
		t.Fatal(err)
	}
	// Real MTA is required for the shortcut email test
	if _, err := net.Dial("tcp", "127.0.0.1:25"); err == nil {
		shortcutMatch := `From howard@localhost Sun Feb 26 18:17:34 2017
Return-Path: <howard@localhost>
X-Original-To: howard@localhost
Delivered-To: howard@localhost
Received: by localhost (Postfix, from userid 1000)
        id 542EA2421BD; Sun, 26 Feb 2017 18:17:34 +0100 (CET)
Date: Sun, 26 Feb 2017 18:17:34 +0100
To: howard@localhost
Subject: hi howard
User-Agent: Heirloom mailx 12.5 7/5/10
MIME-Version: 1.0
Content-Type: text/plain; charset=us-ascii
Content-Transfer-Encoding: 7bit
Message-Id: <20170226171734.542EA2421BD@localhost.>
From: howard@localhost (Howard Guo)
Status: R

PIN mismatch
mailshortcut
`
		if err := mailproc.Process([]byte(shortcutMatch)); err != nil {
			t.Fatal(err)
		}
		t.Log("Check howard@localhost mailbox")
	}

	// ============ Test SMTPD ============
	mailDaemon := config.GetMailDaemon()
	var mailDaemonStoppedNormally bool
	go func() {
		if err := mailDaemon.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
		mailDaemonStoppedNormally = true
	}()
	time.Sleep(3 * time.Second) // this really should be env.HTTPPublicIPTimeout * time.Second
	// Try to exceed rate limit
	testMessage := "Content-type: text/plain; charset=utf-8\r\nFrom: MsgFrom@whatever\r\nTo: MsgTo@whatever\r\nSubject: text subject\r\n\r\ntest body"
	success = 0
	for i := 0; i < 100; i++ {
		if err := smtp.SendMail("127.0.0.1:18573", nil, "ClientFrom@localhost", []string{"ClientTo@localhost"}, []byte(testMessage)); err == nil {
			success++
		}
	}
	if success < 5 || success > 15 {
		t.Fatal("delivered", success)
	}
	time.Sleep(smtpd.RateLimitIntervalSec * time.Second)
	// Send an ordinary mail to the daemon
	mailMsg := "Content-type: text/plain; charset=utf-8\r\nFrom: MsgFrom@whatever\r\nTo: MsgTo@whatever\r\nSubject: text subject\r\n\r\ntest body"
	if err := smtp.SendMail("127.0.0.1:18573", nil, "ClientFrom@localhost", []string{"ClientTo@localhost"}, []byte(mailMsg)); err != nil {
		if err != nil {
			t.Fatal(err)
		}
	}
	// Try run a command via email
	mailMsg = "Content-type: text/plain; charset=utf-8\r\nFrom: MsgFrom@whatever\r\nTo: MsgTo@whatever\r\nSubject: command subject\r\n\r\nmailsecret.s echo hi"
	if err := smtp.SendMail("127.0.0.1:18573", nil, "ClientFrom@localhost", []string{"ClientTo@localhost"}, []byte(mailMsg)); err != nil {
		if err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(3 * time.Second)
	t.Log("Check howard@localhost and root@localhost mailbox")
	// Daemon must stop in a second
	mailDaemon.Stop()
	time.Sleep(1 * time.Second)
	if !mailDaemonStoppedNormally {
		t.Fatal("did not stop")
	}

	// ============ Test sock daemon ============
	sockDaemon := config.GetSockDaemon()
	var stopped bool
	go func() {
		if err := sockDaemon.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
		stopped = true
	}()
	time.Sleep(2 * time.Second)
	if conn, err := net.Dial("tcp", "127.0.0.1:6891"); err != nil {
		t.Fatal(err)
	} else if n, err := conn.Write([]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}); err != nil && n != 10 {
		t.Fatal(err, n)
	}
	sockDaemon.Stop()
	time.Sleep(1 * time.Second)
	if !stopped {
		t.Fatal("did not stop")
	}

	// ============ Test telegram bot ============
	telegramBot := config.GetTelegramBot()
	// It is really difficult to test the chat routine
	// So I am going to only do the API test call
	if err := telegramBot.StartAndBlock(); err == nil || strings.Index(err.Error(), "HTTP") == -1 {
		t.Fatal(err)
	}
}
