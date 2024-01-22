package client

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/soupdiver/creg/adguardhome"
)

type Client struct {
	Auth       string
	Endpoint   *url.URL
	HttpClient http.Client
}

func New(endpoint, auth string) *Client {
	u, err := url.Parse(endpoint)
	if err != nil {
		panic(err)
	}

	c := &Client{
		Auth:     auth,
		Endpoint: u,
		HttpClient: http.Client{
			Timeout: time.Second * 10,
			Transport: http.RoundTripper(&http.Transport{
				MaxIdleConns:        100,
				MaxConnsPerHost:     100,
				MaxIdleConnsPerHost: 100,
				IdleConnTimeout:     time.Second * 90,
			}),
		},
	}

	return c
}

func (c *Client) doRequest(r *http.Request, res interface{}) (*http.Response, error) {
	up := strings.Split(c.Auth, ":")
	r.SetBasicAuth(up[0], up[1])

	r.Header.Add("Content-Type", "application/json")

	resp, err := c.HttpClient.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if res != nil {
		err = json.NewDecoder(resp.Body).Decode(res)
		if err != nil {
			return resp, err
		}
	}
	return resp, nil
}

func (c *Client) List() (adguardhome.RewriteListResponse, error) {
	req, err := http.NewRequest("GET", c.Endpoint.RequestURI()+"/control/rewrite/list", nil)
	if err != nil {
		panic(err)
	}

	var res adguardhome.RewriteListResponse
	_, err = c.doRequest(req, &res)
	if err != nil {
		panic(err)
	}

	return res, nil
}

func (c *Client) Add(in adguardhome.RewriteListResponseItem) {
	c.post("add", in)
}

func (c *Client) Delete(in adguardhome.RewriteListResponseItem) {
	c.post("delete", in)
}

func (c *Client) post(op string, in adguardhome.RewriteListResponseItem) {
	b, err := json.Marshal(in)
	if err != nil {
		panic(err)
	}

	body := bytes.NewReader(b)
	req, err := http.NewRequest("POST", c.Endpoint.String()+"/control/rewrite/"+op, body)
	if err != nil {
		panic(err)
	}

	r, err := c.doRequest(req, nil)
	if err != nil {
		panic(err)
	}
	// log.Printf("status: %s", r.Status)

	if r.StatusCode >= 500 {
		panic("status not ok: " + r.Status)
	}

	// if r.StatusCode != http.StatusOK {
	// 	panic("status not ok: " + r.Status)
	// }
}
