package artifactory

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func (c *ArtifactoryClient) Get(path string, options map[string]string) ([]byte, error) {
	return c.makeRequest("GET", path, options, nil)
}

func (c *ArtifactoryClient) Post(path string, data string, options map[string]string) ([]byte, error) {
	body := strings.NewReader(data)
	return c.makeRequest("POST", path, options, body)
}

func (c *ArtifactoryClient) Put(path string, data string, options map[string]string) ([]byte, error) {
	body := strings.NewReader(strings.TrimSuffix(data, "\n"))
	return c.makeRequest("PUT", path, options, body)
}

func (c *ArtifactoryClient) Delete(path string) error {
	_, err := c.makeRequest("DELETE", path, make(map[string]string), nil)
	if err != nil {
		return err
	} else {
		return nil
	}
}

func (c *ArtifactoryClient) makeRequest(method string, path string, options map[string]string, body io.Reader) ([]byte, error) {
	qs := url.Values{}
	var contentType string
	for q, p := range options {
		if q == "content-type" {
			contentType = p
			delete(options, q)
		} else {
			qs.Add(q, p)
		}
	}
	var base_req_path string
	// swapped out legacy code below for simply trimming the trailing slash
	//if c.Config.BaseURL[:len(c.Config.BaseURL)-1] == "/" {
	//	base_req_path = c.Config.BaseURL + path
	//} else {
	//	base_req_path = c.Config.BaseURL + "/" + path
	//}
	base_req_path = strings.TrimSuffix(c.Config.BaseURL, "/") + path
	u, err := url.Parse(base_req_path)
	if err != nil {
		var data bytes.Buffer
		return data.Bytes(), err
	}
	if len(options) != 0 {
		u.RawQuery = qs.Encode()
	}
	buf := new(bytes.Buffer)
	if body != nil {
		buf.ReadFrom(body)
	}
	req, _ := http.NewRequest(method, u.String(), bytes.NewReader(buf.Bytes()))
	if body != nil {
		h := sha1.New()
		h.Write(buf.Bytes())
		chkSum := h.Sum(nil)
		req.Header.Add("X-Checksum-Sha1", fmt.Sprintf("%x", chkSum))
	}
	req.Header.Add("user-agent", "artifactory-go."+VERSION)
	req.Header.Add("X-Result-Detail", "info, properties")
	if contentType != "" {
		req.Header.Add("Content-Type", contentType)
	}
	if c.Config.AuthMethod == "basic" {
		req.SetBasicAuth(c.Config.Username, c.Config.Password)
	} else {
		req.Header.Add("X-JFrog-Art-Api", c.Config.Token)
	}
	if os.Getenv("ARTIFACTORY_DEBUG") != "" {
		log.Printf("Headers: %#v", req.Header)
		log.Printf("Body: %#v", string(buf.Bytes()))
	}
	r, err := c.Client.Do(req)
	if err != nil {
		var data bytes.Buffer
		return data.Bytes(), err
	} else {
		defer r.Body.Close()
		data, err := ioutil.ReadAll(r.Body)
		if r.StatusCode < 200 || r.StatusCode > 299 {
			var ej ErrorsJson
			uerr := json.Unmarshal(data, &ej)
			if uerr != nil {
				emsg := fmt.Sprintf("Non-2xx code returned: %d. Message follows:\n%s", r.StatusCode, string(data))
				return data, errors.New(emsg)
			} else {
				var emsgs []string
				for _, i := range ej.Errors {
					emsgs = append(emsgs, i.Message)
				}
				return data, errors.New(strings.Join(emsgs, "\n"))
			}
		} else {
			return data, err
		}
	}
}
