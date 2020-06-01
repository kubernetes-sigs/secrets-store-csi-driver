package controllers

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"strings"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
)

// getCertPart returns the certificate or the private key part of the cert
func getCertPart(data []byte, key string) ([]byte, error) {
	if key == corev1.TLSPrivateKeyKey {
		return getPrivateKey(data)
	}
	if key == corev1.TLSCertKey {
		return getCert(data)
	}
	return nil, fmt.Errorf("tls key is not supported. Only tls.key and tls.crt are supported")
}

// getCert returns the certificate part of a cert
func getCert(data []byte) ([]byte, error) {
	var certs []byte
	for {
		pemBlock, rest := pem.Decode(data)
		if pemBlock == nil {
			break
		}
		if pemBlock.Type == certType {
			block := pem.EncodeToMemory(pemBlock)
			certs = append(certs, block...)
		}
		data = rest
	}
	return certs, nil
}

// getPrivateKey returns the private key part of a cert
func getPrivateKey(data []byte) ([]byte, error) {
	var der []byte
	var derKey []byte
	for {
		pemBlock, rest := pem.Decode(data)
		if pemBlock == nil {
			break
		}
		if pemBlock.Type != certType {
			der = pemBlock.Bytes
		}
		data = rest
	}

	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		derKey = x509.MarshalPKCS1PrivateKey(key)
	}

	if key, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		switch key := key.(type) {
		case *rsa.PrivateKey:
			derKey = x509.MarshalPKCS1PrivateKey(key)
		case *ecdsa.PrivateKey:
			derKey, err = x509.MarshalECPrivateKey(key)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unknown private key type found while getting key. Only rsa and ecdsa are supported")
		}
	}
	if key, err := x509.ParseECPrivateKey(der); err == nil {
		derKey, err = x509.MarshalECPrivateKey(key)
		if err != nil {
			return nil, err
		}
	}
	block := &pem.Block{
		Type:  privateKeyType,
		Bytes: derKey,
	}

	return pem.EncodeToMemory(block), nil
}

// getSecretType returns a k8s secret type, defaults to Opaque
func getSecretType(sType string) corev1.SecretType {
	switch sType {
	case "kubernetes.io/basic-auth":
		return corev1.SecretTypeBasicAuth
	case "bootstrap.kubernetes.io/token":
		return corev1.SecretTypeBootstrapToken
	case "kubernetes.io/dockerconfigjson":
		return corev1.SecretTypeDockerConfigJson
	case "kubernetes.io/dockercfg":
		return corev1.SecretTypeDockercfg
	case "kubernetes.io/ssh-auth":
		return corev1.SecretTypeSSHAuth
	case "kubernetes.io/service-account-token":
		return corev1.SecretTypeServiceAccountToken
	case "kubernetes.io/tls":
		return corev1.SecretTypeTLS
	default:
		return corev1.SecretTypeOpaque
	}
}

// getMountedFiles returns all the mounted files names with filepath base as key
func getMountedFiles(targetPath string) (map[string]string, error) {
	paths := make(map[string]string, 0)
	// loop thru all the mounted files
	files, err := ioutil.ReadDir(targetPath)
	if err != nil {
		log.Errorf("failed to list all files in target path %s, err: %v", targetPath, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	sep := "/"
	if strings.HasPrefix(targetPath, "c:\\") {
		sep = "\\"
	} else if strings.HasPrefix(targetPath, `c:\`) {
		sep = `\`
	}
	for _, file := range files {
		paths[file.Name()] = targetPath + sep + file.Name()
	}
	return paths, nil
}
