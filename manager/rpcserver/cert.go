/*
 *     Copyright 2022 The Dragonfly Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package rpcserver

import (
	"context"
	"crypto/rand"
	"crypto/x509"
	"fmt"
	"math/big"
	"net"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	securityv1 "d7y.io/api/pkg/apis/security/v1"

	logger "d7y.io/dragonfly/v2/internal/dflog"
)

func (s *Server) IssueCertificate(ctx context.Context, req *securityv1.CertificateRequest) (*securityv1.CertificateResponse, error) {
	if s.selfSignedCert == nil {
		return nil, status.Errorf(codes.Unavailable, "ca is missing for this manager instance")
	}

	var (
		ip  string
		err error
	)
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "invalid grpc peer info")
	}

	if addr, ok := p.Addr.(*net.TCPAddr); ok {
		ip = addr.IP.String()
	} else {
		ip, _, err = net.SplitHostPort(p.Addr.String())
		if err != nil {
			return nil, err
		}
	}

	// Parse csr.
	csr, err := x509.ParseCertificateRequest(req.Csr)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid csr format: %s", err.Error())
	}

	// Check csr signature.
	// TODO check csr common name and so on.
	if err = csr.CheckSignature(); err != nil {
		return nil, err
	}
	logger.Infof("valid csr: %#v", csr.Subject)

	serial, err := rand.Int(rand.Reader, (&big.Int{}).Exp(big.NewInt(2), big.NewInt(159), nil))
	if err != nil {
		return nil, err
	}

	// TODO only valid for peer ip
	// BTW we need support both of ipv4 and ipv6.
	ips := csr.IPAddresses
	if len(ips) == 0 {
		// Add default connected ip.
		ips = []net.IP{net.ParseIP(ip)}
	}

	now := time.Now()
	template := x509.Certificate{
		SerialNumber:          serial,
		Subject:               csr.Subject,
		DNSNames:              csr.DNSNames,
		EmailAddresses:        csr.EmailAddresses,
		IPAddresses:           ips,
		URIs:                  csr.URIs,
		NotBefore:             now.Add(-10 * time.Minute).UTC(),
		NotAfter:              now.Add(req.ValidityPeriod.AsDuration()).UTC(),
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageDataEncipherment | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}

	cert, err := x509.CreateCertificate(rand.Reader, &template, s.selfSignedCert.X509Cert, csr.PublicKey, s.selfSignedCert.TLSCert.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate certificate, error: %s", err)
	}

	return &securityv1.CertificateResponse{
		CertificateChain: append([][]byte{cert}, s.selfSignedCert.CertChain...),
	}, nil
}
