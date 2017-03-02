package httpd

import (
	"github.com/HouzuoGuo/websh/frontend/common"
	"github.com/HouzuoGuo/websh/frontend/httpd/api"
	"github.com/HouzuoGuo/websh/httpclient"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// TODO: upgrade to go 1.8 and implement graceful httpd shutdown.
func TestHTTPD_StartAndBlock(t *testing.T) {
	// Create a temporary file for index
	indexFile := "/tmp/test-websh-index.html"
	defer os.Remove(indexFile)
	if err := ioutil.WriteFile(indexFile, []byte("this is index"), 0644); err != nil {
		t.Fatal(err)
	}
	// Create a temporary directory of file
	htmlDir := "/tmp/test-websh-dir"
	if err := os.MkdirAll(htmlDir, 0755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(htmlDir)
	if err := ioutil.WriteFile(htmlDir+"/a.html", []byte("a html"), 0644); err != nil {
		t.Fatal(err)
	}

	daemon := HTTPD{
		ListenAddress:      "127.0.0.1",
		ListenPort:         13589, // hard coded port is a random choice
		Processor:          &common.CommandProcessor{},
		ServeIndexDocument: indexFile,
		ServeDirectories:   map[string]string{"my/dir": "/tmp/test-websh-dir"},
		SpecialHandlers: map[string]api.HandlerFactory{
			"/twilio_sms":      &api.HandleTwilioSMSHook{},
			"/twilio_call":     &api.HandleTwilioCallHook{CallbackEndpoint: "/twilio_callback", CallGreeting: "hello"},
			"/twilio_callback": &api.HandleTwilioCallCallback{MyEndpoint: "/twilio_callback"},
			"/test":            &api.HandleFeatureSelfTest{},
		},
	}
	if err := daemon.Initialise(); err == nil || !strings.Contains(err.Error(), common.ErrBadProcessorConfig) {
		t.Fatal("did not error due to insane CommandProcessor")
	}
	daemon.Processor = common.GetTestCommandProcessor()
	if err := daemon.Initialise(); err != nil {
		t.Fatal(err)
	}
	go func() {
		if err := daemon.StartAndBlock(); err != nil {
			t.Fatal(err)
		}
	}()
	time.Sleep(2 * time.Second)

	addr := "http://127.0.0.1:13589"

	// Index handle
	for _, location := range IndexLocations {
		resp, err := httpclient.DoHTTP(httpclient.Request{}, addr+location)
		if err != nil || resp.StatusCode != http.StatusOK || string(resp.Body) != "this is index" {
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
	if err != nil || resp.StatusCode != http.StatusNotFound || len(resp.Body) != 0 {
		t.Fatal(err, string(resp.Body), resp)
	}
	// Specialised handle - self_test
	resp, err = httpclient.DoHTTP(httpclient.Request{}, addr+"/test")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatal(err, string(resp.Body), resp)
	}
}