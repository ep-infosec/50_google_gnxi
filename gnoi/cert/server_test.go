/* Copyright 2018 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cert

import (
	"context"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/gnxi/gnoi/cert/pb"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
)

// Equal checks if two data structures are equal, ignoring protobuf internal variables.
func Equal(x, y interface{}) bool {
	return cmp.Equal(x, y, protocmp.Transform())
}

// Diff returns the diff between two data structures, ignoring protobuf internal variables.
func Diff(x, y interface{}) string {
	return cmp.Diff(x, y, protocmp.Transform())
}

type mockManagerInterface struct {
	ManagerInterface

	mockInstall     func(string, []byte, [][]byte) error
	mockRotate      func(string, []byte, [][]byte) (func(), func(), error)
	mockGenCSR      func(pkix.Name) ([]byte, error)
	mockGetCertInfo func() ([]*Info, error)
	mockRevoke      func([]string) ([]string, map[string]string, error)
}

func (mmi *mockManagerInterface) Install(a string, b []byte, c [][]byte) error {
	return mmi.mockInstall(a, b, c)
}
func (mmi *mockManagerInterface) Rotate(a string, b []byte, c [][]byte) (func(), func(), error) {
	return mmi.mockRotate(a, b, c)
}
func (mmi *mockManagerInterface) GenCSR(a pkix.Name) ([]byte, error) {
	return mmi.mockGenCSR(a)
}
func (mmi *mockManagerInterface) GetCertInfo() ([]*Info, error) {
	return mmi.mockGetCertInfo()
}
func (mmi *mockManagerInterface) Revoke(a []string) ([]string, map[string]string, error) {
	return mmi.mockRevoke(a)
}

type mockInstallServer struct {
	pb.CertificateManagement_InstallServer
	reqMap  []*installRequestMap
	i       int
	recv    chan int
	recvErr chan error
}

func (s *mockInstallServer) Send(resp *pb.InstallCertificateResponse) error {
	if s.i < len(s.reqMap) {
		if reflect.TypeOf(resp.InstallResponse) == reflect.TypeOf(s.reqMap[s.i].resp.InstallResponse) {
			s.recv <- s.i
		} else {
			s.recvErr <- errors.New("error")
		}
		s.i++
	}
	return nil
}

func (s *mockInstallServer) Recv() (*pb.InstallCertificateRequest, error) {
	select {
	case i := <-s.recv:
		return s.reqMap[i].req, nil
	case err := <-s.recvErr:
		return nil, err
	}
}

type mockRotateServer struct {
	pb.CertificateManagement_RotateServer
	reqMap  []*rotateRequestMap
	i       int
	recv    chan int
	recvErr chan error
}

func (s *mockRotateServer) Send(resp *pb.RotateCertificateResponse) error {
	if s.i < len(s.reqMap) {
		if reflect.TypeOf(resp.RotateResponse) == reflect.TypeOf(s.reqMap[s.i].resp.RotateResponse) {
			s.recv <- s.i
		} else {
			s.recvErr <- errors.New("error")
		}
		s.i++
	}
	return nil
}

func (s *mockRotateServer) Recv() (*pb.RotateCertificateRequest, error) {
	select {
	case i := <-s.recv:
		return s.reqMap[i].req, nil
	case err := <-s.recvErr:
		return nil, err
	}
}

func TestTargeRotate(t *testing.T) {
	tests := []struct {
		name   string
		reqMap []*rotateRequestMap
		err    error
	}{
		{
			"expected GenerateCSRRequest, got something else",
			[]*rotateRequestMap{
				{
					req:  nil,
					resp: nil,
				},
			},
			errors.New("expected GenerateCSRRequest, got something else"),
		},
		{
			"certificate not supported",
			[]*rotateRequestMap{
				{
					req: &pb.RotateCertificateRequest{
						RotateRequest: &pb.RotateCertificateRequest_GenerateCsr{GenerateCsr: &pb.GenerateCSRRequest{CsrParams: &pb.CSRParams{}}},
					},
					resp: nil,
				},
			},
			errors.New("certificate type \"CT_UNKNOWN\" not supported"),
		},
		{
			"key not supported",
			[]*rotateRequestMap{
				{
					req: &pb.RotateCertificateRequest{
						RotateRequest: &pb.RotateCertificateRequest_GenerateCsr{GenerateCsr: &pb.GenerateCSRRequest{CsrParams: &pb.CSRParams{Type: 1}}},
					},
					resp: nil,
				},
			},
			errors.New("key type \"KT_UNKNOWN\" not supported"),
		},
		{
			"failed to receive RotateCertificateRequest",
			[]*rotateRequestMap{
				{
					req: &pb.RotateCertificateRequest{
						RotateRequest: &pb.RotateCertificateRequest_GenerateCsr{GenerateCsr: &pb.GenerateCSRRequest{CsrParams: &pb.CSRParams{Type: 1, KeyType: 1}}},
					},
				},
				{
					resp: &pb.RotateCertificateResponse{},
				},
			},
			errors.New("failed to receive RotateCertificateRequest: error"),
		},
		{
			"expected LoadCertificateRequest, got something else",
			[]*rotateRequestMap{
				{
					req: &pb.RotateCertificateRequest{
						RotateRequest: &pb.RotateCertificateRequest_GenerateCsr{GenerateCsr: &pb.GenerateCSRRequest{CsrParams: &pb.CSRParams{Type: 1, KeyType: 1}}},
					},
				},
				{
					resp: &pb.RotateCertificateResponse{
						RotateResponse: &pb.RotateCertificateResponse_GeneratedCsr{},
					},
				},
			},
			errors.New("expected LoadCertificateRequest, got something else"),
		},
		{
			"unexpected Certificate type",
			[]*rotateRequestMap{
				{
					req: &pb.RotateCertificateRequest{
						RotateRequest: &pb.RotateCertificateRequest_GenerateCsr{GenerateCsr: &pb.GenerateCSRRequest{CsrParams: &pb.CSRParams{Type: 1, KeyType: 1}}},
					},
				},
				{
					resp: &pb.RotateCertificateResponse{
						RotateResponse: &pb.RotateCertificateResponse_GeneratedCsr{},
					},
					req: &pb.RotateCertificateRequest{
						RotateRequest: &pb.RotateCertificateRequest_LoadCertificate{LoadCertificate: &pb.LoadCertificateRequest{Certificate: &pb.Certificate{Type: 0}}},
					},
				},
			},
			errors.New("unexpected Certificate type: 0"),
		},
		{
			"rolling back - failed to receive RotateCertificateRequest",
			[]*rotateRequestMap{
				{
					req: &pb.RotateCertificateRequest{
						RotateRequest: &pb.RotateCertificateRequest_GenerateCsr{GenerateCsr: &pb.GenerateCSRRequest{CsrParams: &pb.CSRParams{Type: 1, KeyType: 1}}},
					},
				},
				{
					resp: &pb.RotateCertificateResponse{
						RotateResponse: &pb.RotateCertificateResponse_GeneratedCsr{},
					},
					req: &pb.RotateCertificateRequest{
						RotateRequest: &pb.RotateCertificateRequest_LoadCertificate{LoadCertificate: &pb.LoadCertificateRequest{
							Certificate:    &pb.Certificate{Type: 1},
							CaCertificates: []*pb.Certificate{{Type: pb.CertificateType_CT_X509}}},
						},
					},
				},
				{
					resp: &pb.RotateCertificateResponse{
						RotateResponse: &pb.RotateCertificateResponse_GeneratedCsr{},
					},
				},
			},
			errors.New("rolling back - failed to receive RotateCertificateRequest: error"),
		},
		{
			"expected FinalizeRequest, got something else",
			[]*rotateRequestMap{
				{
					req: &pb.RotateCertificateRequest{
						RotateRequest: &pb.RotateCertificateRequest_GenerateCsr{GenerateCsr: &pb.GenerateCSRRequest{CsrParams: &pb.CSRParams{Type: 1, KeyType: 1}}},
					},
				},
				{
					resp: &pb.RotateCertificateResponse{
						RotateResponse: &pb.RotateCertificateResponse_GeneratedCsr{},
					},
					req: &pb.RotateCertificateRequest{
						RotateRequest: &pb.RotateCertificateRequest_LoadCertificate{LoadCertificate: &pb.LoadCertificateRequest{
							Certificate:    &pb.Certificate{Type: 1},
							CaCertificates: []*pb.Certificate{{Type: pb.CertificateType_CT_X509}}},
						},
					},
				},
				{
					resp: &pb.RotateCertificateResponse{
						RotateResponse: &pb.RotateCertificateResponse_LoadCertificate{},
					},
				},
			},
			errors.New("expected FinalizeRequest, got something else"),
		},
		{
			"terminate",
			[]*rotateRequestMap{
				{
					req: &pb.RotateCertificateRequest{
						RotateRequest: &pb.RotateCertificateRequest_GenerateCsr{GenerateCsr: &pb.GenerateCSRRequest{CsrParams: &pb.CSRParams{Type: 1, KeyType: 1}}},
					},
				},
				{
					resp: &pb.RotateCertificateResponse{
						RotateResponse: &pb.RotateCertificateResponse_GeneratedCsr{},
					},
					req: &pb.RotateCertificateRequest{
						RotateRequest: &pb.RotateCertificateRequest_LoadCertificate{LoadCertificate: &pb.LoadCertificateRequest{
							Certificate:    &pb.Certificate{Type: 1},
							CaCertificates: []*pb.Certificate{{Type: pb.CertificateType_CT_X509}}},
						},
					},
				},
				{
					resp: &pb.RotateCertificateResponse{
						RotateResponse: &pb.RotateCertificateResponse_LoadCertificate{},
					},
					req: &pb.RotateCertificateRequest{
						RotateRequest: &pb.RotateCertificateRequest_FinalizeRotation{FinalizeRotation: &pb.FinalizeRequest{}},
					},
				},
			},
			nil,
		},
	}
	mmi := &mockManagerInterface{
		mockGenCSR: func(p pkix.Name) ([]byte, error) {
			return []byte{}, nil
		},
		mockRotate: func(string, []byte, [][]byte) (func(), func(), error) {
			return func() {}, func() {}, nil
		},
	}
	s := NewServer(mmi)
	for _, test := range tests {
		stream := &mockRotateServer{
			i:       1,
			reqMap:  test.reqMap,
			recv:    make(chan int, 1),
			recvErr: make(chan error, 1),
		}
		stream.recv <- 0
		if err := s.Rotate(stream); fmt.Sprintf("%v", err) != fmt.Sprintf("%v", test.err) {
			t.Errorf("wanted err(%v), got(%v)", test.err, err)
		}
	}
}

func TestTargetInstall(t *testing.T) {
	tests := []struct {
		name   string
		reqMap []*installRequestMap
		err    error
	}{
		{
			"expected GenerateCSRRequest, got something else",
			[]*installRequestMap{
				{
					req:  nil,
					resp: nil,
				},
			},
			errors.New("expected GenerateCSRRequest, got something else"),
		},
		{
			"certificate not supported",
			[]*installRequestMap{
				{
					req: &pb.InstallCertificateRequest{
						InstallRequest: &pb.InstallCertificateRequest_GenerateCsr{GenerateCsr: &pb.GenerateCSRRequest{CsrParams: &pb.CSRParams{}}},
					},
					resp: nil,
				},
			},
			errors.New("certificate type \"CT_UNKNOWN\" not supported"),
		},
		{
			"failed to receive InstallCertificateRequest",
			[]*installRequestMap{
				{
					req: &pb.InstallCertificateRequest{
						InstallRequest: &pb.InstallCertificateRequest_GenerateCsr{GenerateCsr: &pb.GenerateCSRRequest{CsrParams: &pb.CSRParams{Type: 1, KeyType: 1}}},
					},
				},
				{
					resp: &pb.InstallCertificateResponse{
						InstallResponse: &pb.InstallCertificateResponse_LoadCertificate{},
					},
				},
			},
			errors.New("failed to receive InstallCertificateRequest: error"),
		},
		{
			"expected LoadCertificateRequest, got something else",
			[]*installRequestMap{
				{
					req: &pb.InstallCertificateRequest{
						InstallRequest: &pb.InstallCertificateRequest_GenerateCsr{GenerateCsr: &pb.GenerateCSRRequest{CsrParams: &pb.CSRParams{Type: 1, KeyType: 1}}},
					},
				},
				{
					resp: &pb.InstallCertificateResponse{
						InstallResponse: &pb.InstallCertificateResponse_GeneratedCsr{GeneratedCsr: &pb.GenerateCSRResponse{Csr: &pb.CSR{Type: 1}}},
					},
					req: &pb.InstallCertificateRequest{InstallRequest: nil},
				},
			},
			errors.New("expected LoadCertificateRequest, got something else"),
		},
		{
			"unexpected Certificate type",
			[]*installRequestMap{
				{
					req: &pb.InstallCertificateRequest{
						InstallRequest: &pb.InstallCertificateRequest_GenerateCsr{GenerateCsr: &pb.GenerateCSRRequest{CsrParams: &pb.CSRParams{Type: 1, KeyType: 1}}},
					},
				},
				{
					resp: &pb.InstallCertificateResponse{
						InstallResponse: &pb.InstallCertificateResponse_GeneratedCsr{GeneratedCsr: &pb.GenerateCSRResponse{Csr: &pb.CSR{Type: 1}}},
					},
					req: &pb.InstallCertificateRequest{InstallRequest: &pb.InstallCertificateRequest_LoadCertificate{
						LoadCertificate: &pb.LoadCertificateRequest{Certificate: &pb.Certificate{Type: pb.CertificateType_CT_UNKNOWN}},
					}},
				},
			},
			errors.New("unexpected Certificate type: \"CT_UNKNOWN\""),
		},
		{
			"unexpected CA Certificate type",
			[]*installRequestMap{
				{
					req: &pb.InstallCertificateRequest{
						InstallRequest: &pb.InstallCertificateRequest_GenerateCsr{GenerateCsr: &pb.GenerateCSRRequest{CsrParams: &pb.CSRParams{Type: 1, KeyType: 1}}},
					},
				},
				{
					resp: &pb.InstallCertificateResponse{
						InstallResponse: &pb.InstallCertificateResponse_GeneratedCsr{GeneratedCsr: &pb.GenerateCSRResponse{Csr: &pb.CSR{Type: 1}}},
					},
					req: &pb.InstallCertificateRequest{InstallRequest: &pb.InstallCertificateRequest_LoadCertificate{
						LoadCertificate: &pb.LoadCertificateRequest{Certificate: &pb.Certificate{Type: pb.CertificateType_CT_X509}, CaCertificates: []*pb.Certificate{{Type: pb.CertificateType_CT_UNKNOWN}}},
					}},
				},
			},
			errors.New("unexpected Certificate type: \"CT_UNKNOWN\""),
		},
		{
			"terminates",
			[]*installRequestMap{
				{
					req: &pb.InstallCertificateRequest{
						InstallRequest: &pb.InstallCertificateRequest_GenerateCsr{GenerateCsr: &pb.GenerateCSRRequest{CsrParams: &pb.CSRParams{Type: 1, KeyType: 1}}},
					},
				},
				{
					resp: &pb.InstallCertificateResponse{
						InstallResponse: &pb.InstallCertificateResponse_GeneratedCsr{GeneratedCsr: &pb.GenerateCSRResponse{Csr: &pb.CSR{Type: 1}}},
					},
					req: &pb.InstallCertificateRequest{InstallRequest: &pb.InstallCertificateRequest_LoadCertificate{
						LoadCertificate: &pb.LoadCertificateRequest{Certificate: &pb.Certificate{Type: pb.CertificateType_CT_X509}, CaCertificates: []*pb.Certificate{{Type: pb.CertificateType_CT_X509}}},
					}},
				},
				{
					resp: &pb.InstallCertificateResponse{
						InstallResponse: &pb.InstallCertificateResponse_LoadCertificate{},
					},
				},
			},
			nil,
		},
	}
	mmi := &mockManagerInterface{
		mockGenCSR: func(p pkix.Name) ([]byte, error) {
			return []byte{}, nil
		},
		mockInstall: func(string, []byte, [][]byte) error {
			return nil
		},
	}
	s := NewServer(mmi)
	for _, test := range tests {
		stream := &mockInstallServer{
			i:       1,
			reqMap:  test.reqMap,
			recv:    make(chan int, 1),
			recvErr: make(chan error, 1),
		}
		stream.recv <- 0
		if err := s.Install(stream); fmt.Sprintf("%v", err) != fmt.Sprintf("%v", test.err) {
			t.Errorf("wanted err(%v), got(%v)", test.err, err)
		}
	}
}

func TestGetCertificates(t *testing.T) {
	mmi := &mockManagerInterface{}
	s := NewServer(mmi)
	ctx := context.Background()
	now := time.Now()
	certPEMEncoder = func(a *x509.Certificate) []byte { return nil }

	tests := []struct {
		in             *pb.GetCertificatesRequest
		mmiGetCertInfo func() ([]*Info, error)
		want           *pb.GetCertificatesResponse
		watErr         bool
	}{
		{
			in: &pb.GetCertificatesRequest{},
			mmiGetCertInfo: func() ([]*Info, error) {
				return nil, fmt.Errorf("some error")
			},
			want:   nil,
			watErr: true,
		},
		{
			in: &pb.GetCertificatesRequest{},
			mmiGetCertInfo: func() ([]*Info, error) {
				return []*Info{
					{certID: "id1", updated: now},
				}, nil
			},
			want: &pb.GetCertificatesResponse{
				CertificateInfo: []*pb.CertificateInfo{
					{
						CertificateId:    "id1",
						ModificationTime: now.UnixNano(),
						Certificate: &pb.Certificate{
							Type:        pb.CertificateType_CT_X509,
							Certificate: nil,
						},
					},
				},
			},
			watErr: false,
		},
	}
	for _, test := range tests {
		mmi.mockGetCertInfo = test.mmiGetCertInfo
		got, err := s.GetCertificates(ctx, test.in)
		if err != nil && !test.watErr {
			t.Errorf("GetCertificates error: %s", err)
		} else if err == nil && test.watErr {
			t.Error("GetCertificates want error, got none")
		}
		if !Equal(test.want, got) {
			t.Errorf("GetCertificates: (-want +got):\n%s", Diff(test.want, got))
		}
	}
}

func TestRevokeCertificates(t *testing.T) {
	mmi := &mockManagerInterface{}
	s := NewServer(mmi)
	ctx := context.Background()

	tests := []struct {
		in        *pb.RevokeCertificatesRequest
		mmiRevoke func([]string) ([]string, map[string]string, error)
		want      *pb.RevokeCertificatesResponse
		watErr    bool
	}{
		{
			in: &pb.RevokeCertificatesRequest{},
			mmiRevoke: func(a []string) ([]string, map[string]string, error) {
				return nil, nil, fmt.Errorf("some error")
			},
			want:   nil,
			watErr: true,
		},
		{
			in: &pb.RevokeCertificatesRequest{
				CertificateId: []string{"id1", "id2"},
			},
			mmiRevoke: func(a []string) ([]string, map[string]string, error) {
				return []string{"id1"}, map[string]string{"id2": "some error"}, nil
			},
			want: &pb.RevokeCertificatesResponse{
				RevokedCertificateId: []string{"id1"},
				CertificateRevocationError: []*pb.CertificateRevocationError{
					{CertificateId: "id2", ErrorMessage: "some error"},
				},
			},
			watErr: false,
		},
	}
	for _, test := range tests {
		mmi.mockRevoke = test.mmiRevoke
		got, err := s.RevokeCertificates(ctx, test.in)
		if err != nil && !test.watErr {
			t.Errorf("RevokeCertificates error: %s", err)
		} else if err == nil && test.watErr {
			t.Error("RevokeCertificates want error, got none")
		}
		if !Equal(test.want, got) {
			t.Errorf("RevokeCertificates: (-want +got):\n%s", Diff(test.want, got))
		}
	}
}

func TestCanGenerateCSR(t *testing.T) {
	mmi := &mockManagerInterface{}
	s := NewServer(mmi)
	ctx := context.Background()

	tests := []struct {
		in   *pb.CanGenerateCSRRequest
		want *pb.CanGenerateCSRResponse
	}{
		{
			in: &pb.CanGenerateCSRRequest{
				KeySize:         64,
				KeyType:         pb.KeyType_KT_RSA,
				CertificateType: pb.CertificateType_CT_X509,
			},
			want: &pb.CanGenerateCSRResponse{CanGenerate: false},
		},
		{
			in: &pb.CanGenerateCSRRequest{
				KeySize:         2048,
				KeyType:         pb.KeyType_KT_RSA,
				CertificateType: pb.CertificateType_CT_X509,
			},
			want: &pb.CanGenerateCSRResponse{CanGenerate: true},
		},
		{
			in: &pb.CanGenerateCSRRequest{
				KeySize:         2048,
				KeyType:         pb.KeyType_KT_UNKNOWN,
				CertificateType: pb.CertificateType_CT_X509,
			},
			want: &pb.CanGenerateCSRResponse{CanGenerate: false},
		},
		{
			in: &pb.CanGenerateCSRRequest{
				KeySize:         2048,
				KeyType:         pb.KeyType_KT_RSA,
				CertificateType: pb.CertificateType_CT_UNKNOWN,
			},
			want: &pb.CanGenerateCSRResponse{CanGenerate: false},
		},
	}
	for _, test := range tests {
		got, err := s.CanGenerateCSR(ctx, test.in)
		if err != nil {
			t.Errorf("CanGenerateCSR error: %s", err)
		}
		if !Equal(test.want, got) {
			t.Errorf("CanGenerateCSR: (-want +got):\n%s", Diff(test.want, got))
		}
	}
}
