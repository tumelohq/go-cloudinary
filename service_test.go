// Copyright 2013 Mathias Monnerville and Anthony Baillard.
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package cloudinary

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"net/url"
	"os"
	"testing"
)

func TestDial(t *testing.T) {
	if _, err := Dial("baduri::"); err == nil {
		t.Error("should fail on bad uri")
	}

	// Not a cloudinary:// URL scheme
	if _, err := Dial("http://localhost"); err == nil {
		t.Error("should fail if URL scheme different from cloudinary://")
	}

	// Missing API secret (password)?
	if _, err := Dial("cloudinary://login@cloudname"); err == nil {
		t.Error("should fail when no API secret is provided")
	}

	k := &Service{
		cloudName: "cloudname",
		apiKey:    "login",
		apiSecret: "secret",
	}
	s, err := Dial(fmt.Sprintf("cloudinary://%s:%s@%s", k.apiKey, k.apiSecret, k.cloudName))
	if err != nil {
		t.Error("expect a working service at this stage but got an error.")
	}
	if s.cloudName != k.cloudName || s.apiKey != k.apiKey || s.apiSecret != k.apiSecret {
		t.Errorf("wrong service instance. Expect %v, got %v", k, s)
	}
	uexp := fmt.Sprintf("%s/%s/image/upload/", baseUploadURL, s.cloudName)
	if s.uploadURI.String() != uexp {
		t.Errorf("wrong upload URI. Expect %s, got %s", uexp, s.uploadURI.String())
	}
}

func TestUploadByFile(t *testing.T) {
	s, err := Dial(os.Getenv("CLOUDINARY"))
	if err != nil {
		t.Fatal(err)
	}

	f, err := os.Open("test_logo.png")
	if err != nil {
		t.Fatal(err)
	}

	id, err := s.UploadImageFile(f, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(id)
}

func TestUploadByURL(t *testing.T) {
	s, err := Dial(os.Getenv("CLOUDINARY"))
	if err != nil {
		t.Fatal(err)
	}

	imgURL, err := url.Parse("https://en.wikipedia.org/w/skins/Vector/images/user-avatar.svg?b7f58")
	if err != nil {
		t.Fatal(err)
	}

	id, err := s.UploadImageURL(imgURL, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(id)
}

func goldenImageURL() *url.URL {
	return &url.URL{
		Scheme: "https",
		Host:   "res.cloudinary.com",
		Path:   "/yourcloudname/image/upload/v1579703365/q8zrn0wevsuj30albned.png",
	}
}

func TestGetResizedImageURL(t *testing.T) {
	tests := []struct {
		name     string
		imageURL *url.URL
		size     size
		wantURL  string
		wantErr  bool
	}{
		{
			name:     "happy path",
			imageURL: goldenImageURL(),
			size: size{
				width:  100,
				height: 200,
			},
			wantURL: "https://res.cloudinary.com/yourcloudname/image/upload/w_100,h_200,c_fit/v1579703365/q8zrn0wevsuj30albned.png",
		},
		{
			name: "happy path",
			imageURL: func() *url.URL {
				a := goldenImageURL()
				a.Path = "/yourcloudname/image/upload/img.jpg"
				return a
			}(),
			size: size{
				width:  100,
				height: 200,
			},
			wantURL: "https://res.cloudinary.com/yourcloudname/image/upload/w_100,h_200,c_fit/img.jpg",
		},
		{
			name: "error - bad hostname",
			imageURL: func() *url.URL {
				a := goldenImageURL()
				a.Host = "blah"
				return a
			}(),
			size: size{
				width:  100,
				height: 200,
			},
			wantErr: true,
		},
		{
			name: "error - bad path",
			imageURL: func() *url.URL {
				a := goldenImageURL()
				a.Path = "yourcloudname/blah/laaa.png"
				return a
			}(),
			size: size{
				width:  100,
				height: 200,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			k := &Service{
				cloudName: "cloudname",
				apiKey:    "login",
				apiSecret: "secret",
			}

			//When
			resized, err := k.GetResizedImageURL(tt.imageURL, tt.size)

			//Should
			if (err != nil) != tt.wantErr {
				t.Fatalf("expecting an error: %v. error is: %v", tt.wantErr, err)
			}

			if resized == nil {
				assert.Equal(t, tt.wantURL, "")
			} else {
				assert.Equal(t, tt.wantURL, resized.String())
			}
		})
	}
}
