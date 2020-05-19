/*
Copyright 2020 Betsson Group.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package api

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Options keeps some configuration options for Dex client
type Options struct {
	// HostAndPort host name and port of gRPC server
	HostAndPort string
	// ClientCrt TLS certificate for gRPC client
	ClientCrt string
	// ClientKey TLS certificate key for gRPC client
	ClientKey string
	// ClientCA self signed CA certificate for gRPC TLS connection
	ClientCA string
}

// APIClient represent a client wrapper for Dex
type APIClient struct {
	dex DexClient
}

// NewClient creates a new Dex client
func NewClient(opts *Options) (*APIClient, error) {
	certPool := x509.NewCertPool()
	caCert, err := ioutil.ReadFile(opts.ClientCA) // #nosec
	if err != nil {
		return nil, errors.Wrapf(err, "reading the public CA cert from %q", opts.ClientCA)
	}
	appended := certPool.AppendCertsFromPEM(caCert)
	if !appended {
		return nil, errors.New("failed to append the CA cert to the certs pool")
	}

	clientCert, err := tls.LoadX509KeyPair(opts.ClientCrt, opts.ClientKey)
	if err != nil {
		return nil, errors.Wrapf(err, "loading the client cert %q and private key %q",
			opts.ClientCrt, opts.ClientKey)
	}

	clientTLSConfig := &tls.Config{
		RootCAs:      certPool,
		Certificates: []tls.Certificate{clientCert},
	}
	creds := credentials.NewTLS(clientTLSConfig)

	conn, err := grpc.Dial(opts.HostAndPort, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, errors.Wrapf(err, "opening the gRPC connection with server %q", opts.HostAndPort)
	}
	return &APIClient{
		dex: NewDexClient(conn),
	}, nil
}

// GetServerInfo returns server info
func (c *APIClient) GetServerInfo(ctx context.Context) (string, error) {
	req := &VersionReq{}
	res, err := c.dex.GetVersion(ctx, req)
	if err != nil {
		return "", errors.Wrap(err, "failed to to get DEX version")
	}
	return res.Server, nil
}

// CreateClient a new OIDC client in Dex
func (c *APIClient) CreateClient(ctx context.Context, redirectUris []string, trustedPeers []string,
	public bool, name string, id string, logoURL string, secret string) (*Client, error) {
	req := &CreateClientReq{
		Client: &Client{
			RedirectUris: redirectUris,
			TrustedPeers: trustedPeers,
			Public:       public,
			Name:         name,
			LogoUrl:      logoURL,
			Secret:       secret,
			Id:           id,
		},
	}

	res, err := c.dex.CreateClient(ctx, req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create the OIDC client")
	}

	if res.AlreadyExists {
		return nil, errors.Errorf("client %q already exists", id)
	}

	return res.Client, nil
}

// UpdateClient updates an already registered OIDC client
func (c *APIClient) UpdateClient(ctx context.Context, clientID string, redirectUris []string,
	trustedPeers []string, public bool, name string, logoURL string) error {
	req := &UpdateClientReq{
		Id:           clientID,
		RedirectUris: redirectUris,
		TrustedPeers: trustedPeers,
		Name:         name,
		LogoUrl:      logoURL,
	}
	res, err := c.dex.UpdateClient(ctx, req)
	if err != nil {
		return errors.Wrapf(err, "failed to update the client with id %q", clientID)
	}

	if res.NotFound {
		return fmt.Errorf("update did not find the client with id %q", clientID)
	}
	return nil
}

// DeleteClient deletes the client with given Id from Dex
func (c *APIClient) DeleteClient(ctx context.Context, id string) error {
	req := &DeleteClientReq{
		Id: id,
	}
	res, err := c.dex.DeleteClient(ctx, req)
	if err != nil {
		return errors.Wrapf(err, "failed to delete the client with id %q", id)
	}
	if res.NotFound {
		return fmt.Errorf("delete did not find the client with id %q", id)
	}
	return nil
}
