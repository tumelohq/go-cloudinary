// Copyright 2013 Mathias Monnerville and Anthony Baillard.
// Modified 2020 Simon Partridge & Benjamin King
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package cloudinary provides support for managing static assets
// on the Cloudinary service.
//
// The Cloudinary service allows image and raw files management in
// the cloud.
package cloudinary

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	baseUploadURL = "https://api.cloudinary.com/v1_1"
)

// Service is the cloudinary service
// it allows uploading of images to cloudinary
type Service struct {
	client    http.Client
	cloudName string
	apiKey    string
	apiSecret string
	uploadURI *url.URL // To upload resources
	adminURI  *url.URL // To use the admin API
}

// Upload response after uploading a file.
type uploadResponse struct {
	PublicID     string `json:"public_id"`
	SecureURL    string `json:"secure_url"`
	Version      uint   `json:"version"`
	Format       string `json:"format"`
	ResourceType string `json:"resource_type"` // "image" or "raw"
	Size         int    `json:"bytes"`         // In bytes
}

// Our request type for a request being built
type request struct {
	uri string
	buf *bytes.Buffer
	w   *multipart.Writer
}

// Dial will use the url to connect to the Cloudinary service.
// The uri parameter must be a valid URI with the cloudinary:// scheme,
// e.g.
//  cloudinary://api_key:api_secret@cloud_name
func Dial(uri string) (*Service, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "cloudinary" {
		return nil, errors.New("Missing cloudinary:// scheme in URI")
	}
	secret, exists := u.User.Password()
	if !exists {
		return nil, errors.New("no API secret provided in URI")
	}
	s := &Service{
		client:    http.Client{},
		cloudName: u.Host,
		apiKey:    u.User.Username(),
		apiSecret: secret,
	}
	// Default upload URI to the service. Can change at runtime in the
	// Upload() function for raw file uploading.
	up, err := url.Parse(fmt.Sprintf("%s/%s/image/upload/", baseUploadURL, s.cloudName))
	if err != nil {
		return nil, err
	}
	s.uploadURI = up

	return s, nil
}

// CloudName returns the cloud name used to access the Cloudinary service.
func (s *Service) CloudName() string {
	return s.cloudName
}

// DefaultUploadURI returns the default URI used to upload images to the Cloudinary service.
func (s *Service) DefaultUploadURI() *url.URL {
	return s.uploadURI
}

// UploadImageFile will upload a file to cloudinary
func (s *Service) UploadImageFile(data io.Reader, filename string) (publicID *url.URL, err error) {
	req, err := newRequest(s.DefaultUploadURI().String(), s.apiKey, s.apiSecret)
	if err != nil {
		return nil, err
	}

	if err = req.addImageFileToRequest(data); err != nil {
		return nil, err
	}

	return s.doRequest(req)
}

// UploadImageURL will add an image to cloudinary when given a URL to the image
func (s *Service) UploadImageURL(URL *url.URL, filename string) (publicID *url.URL, err error) {
	req, err := newRequest(s.DefaultUploadURI().String(), s.apiKey, s.apiSecret)
	if err != nil {
		return nil, err
	}

	if err = req.addImageURLToRequest(URL); err != nil {
		return nil, err
	}

	return s.doRequest(req)
}

// GetResizedImageURL will take a URL to an original image and return a URL to a resized version of it
func (s *Service) GetResizedImageURL(ID *url.URL, width, height int) (publicID *url.URL, err error) {
	path := ID.Path
	pathParts := strings.Split(path, "/")
	if pathParts[2] != "image" || pathParts[3] != "upload" {
		return nil, errors.New("url must be of format https://res.cloudinary.com/<cloudName>/image/upload/")
	}

	resizedParams := fmt.Sprintf("w_%d,h_%d,c_fit", width, height)

	//insert params after position 4
	newPath := append(pathParts[:4], append([]string{resizedParams}, pathParts[4:]...)...)

	ID.Path = strings.Join(newPath, "/")

	return ID, nil

}

func newRequest(uri, apiKey, apiSecret string) (*request, error) {
	buf := new(bytes.Buffer)
	w := multipart.NewWriter(buf)

	// Write API key
	ak, err := w.CreateFormField("api_key")
	if err != nil {
		return nil, err
	}
	ak.Write([]byte(apiKey))

	// Write timestamp
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	ts, err := w.CreateFormField("timestamp")
	if err != nil {
		return nil, err
	}
	ts.Write([]byte(timestamp))

	// Write signature
	// BEWARE the generation of signatures is quite particular
	// See this https://cloudinary.com/documentation/upload_images#generating_authentication_signatures
	hash := sha1.New()
	part := fmt.Sprintf("timestamp=%s%s", timestamp, apiSecret)

	io.WriteString(hash, part)
	signature := fmt.Sprintf("%x", hash.Sum(nil))

	si, err := w.CreateFormField("signature")
	if err != nil {
		return nil, err
	}
	si.Write([]byte(signature))

	return &request{
		buf: buf,
		w:   w,
		uri: uri,
	}, nil
}

func (r *request) addImageFileToRequest(data io.Reader) error {
	fw, err := r.w.CreateFormFile("file", "file")
	if err != nil {
		return err
	}

	tmp, err := ioutil.ReadAll(data)
	if err != nil {
		return err
	}
	_, err = fw.Write(tmp)
	return err
}

func (r *request) addImageURLToRequest(url *url.URL) error {
	return r.w.WriteField("file", url.String())
}

func (r *request) buildHTTPRequest() (req *http.Request, closer func() error, err error) {
	err = r.w.Close()
	if err != nil {
		return nil, nil, err
	}

	req, err = http.NewRequest(http.MethodPost, r.uri, r.buf)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", r.w.FormDataContentType())

	return req, req.Body.Close, nil
}

func (s *Service) doRequest(req *request) (*url.URL, error) {
	HTTPreq, closeReq, err := req.buildHTTPRequest()
	if err != nil {
		return nil, err
	}
	defer closeReq()

	resp, err := s.client.Do(HTTPreq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("Request error: " + resp.Status + " Cld Err: " + resp.Header.Get("X-ClD-Error"))
	}

	dec := json.NewDecoder(resp.Body)
	var upInfo uploadResponse
	if err := dec.Decode(&upInfo); err != nil {
		return nil, err
	}

	imgURL, err := url.Parse(upInfo.SecureURL)
	if err != nil {
		return nil, err
	}

	return imgURL, nil
}

func handleHTTPResponse(resp *http.Response) (map[string]interface{}, error) {
	if resp == nil {
		return nil, errors.New("nil http response")
	}
	dec := json.NewDecoder(resp.Body)
	var msg interface{}
	if err := dec.Decode(&msg); err != nil {
		return nil, err
	}
	m := msg.(map[string]interface{})
	if resp.StatusCode != http.StatusOK {
		// JSON error looks like {"error":{"message":"Missing required parameter - public_id"}}
		if e, ok := m["error"]; ok {
			return nil, errors.New(e.(map[string]interface{})["message"].(string))
		}
		return nil, errors.New(resp.Status)
	}
	return m, nil
}

// Delete deletes a resource uploaded to Cloudinary.
func (s *Service) Delete(publicURL url.URL) error {
	return errors.New("Not implemented")
	// publicID := publicURL.Path

	// timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	// data := url.Values{
	// 	"api_key":   []string{s.apiKey},
	// 	"public_id": []string{publicID.String()},
	// 	"timestamp": []string{timestamp},
	// }

	// // Signature
	// hash := sha1.New()
	// part := fmt.Sprintf("public_id=%s&timestamp=%s%s", publicID, timestamp, s.apiSecret)
	// io.WriteString(hash, part)
	// data.Set("signature", fmt.Sprintf("%x", hash.Sum(nil)))

	// resp, err := http.PostForm(fmt.Sprintf("%s/%s/image/destroy/", baseUploadURL, s.cloudName), data)
	// if err != nil {
	// 	return err
	// }

	// m, err := handleHTTPResponse(resp)
	// if err != nil {
	// 	return err
	// }
	// if e, ok := m["result"]; ok {
	// 	fmt.Println(e.(string))
	// }

	// return nil
}
