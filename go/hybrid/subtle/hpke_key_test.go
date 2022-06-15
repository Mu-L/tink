// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
////////////////////////////////////////////////////////////////////////////////

package subtle_test

import (
	"bytes"
	"testing"

	"google.golang.org/protobuf/proto"
	"github.com/google/tink/go/hybrid"
	"github.com/google/tink/go/hybrid/subtle"
	"github.com/google/tink/go/keyset"
	"github.com/google/tink/go/subtle/random"
	"github.com/google/tink/go/testutil"
	hpkepb "github.com/google/tink/go/proto/hpke_go_proto"
	tinkpb "github.com/google/tink/go/proto/tink_go_proto"
)

func TestPublicKeyFromPrimaryKey(t *testing.T) {
	keyTemplate := hybrid.DHKEM_X25519_HKDF_SHA256_HKDF_SHA256_CHACHA20_POLY1305_Raw_Key_Template()
	privKH, err := keyset.NewHandle(keyTemplate)
	if err != nil {
		t.Fatalf("NewHandle(%v) err = %v, want nil", keyTemplate, err)
	}
	pubKH, err := privKH.Public()
	if err != nil {
		t.Fatalf("Public() err = %v, want nil", err)
	}

	pubKeyBytes, err := subtle.PublicKeyFromPrimaryKey(pubKH, keyTemplate)
	if err != nil {
		t.Fatalf("PublicKeyFromPrimaryKey(%v) err = %v, want nil", pubKH, err)
	}

	// Construct a public key handle with the public key bytes.
	typeURL := "type.googleapis.com/google.crypto.tink.HpkePublicKey"
	kd, err := keyDataFromBytes(t, pubKeyBytes, hpkepb.HpkeAead_CHACHA20_POLY1305, typeURL)
	if err != nil {
		t.Fatalf("keyDataFromBytes(%v, %v, %v) err = %v, want nil", pubKeyBytes, hpkepb.HpkeAead_CHACHA20_POLY1305, typeURL, err)
	}
	ks := testutil.NewKeyset(12345, []*tinkpb.Keyset_Key{
		// Primary key.
		testutil.NewKey(kd, tinkpb.KeyStatusType_ENABLED, 12345, tinkpb.OutputPrefixType_RAW),
		// Non-primary key.
		testutil.NewKey(kd, tinkpb.KeyStatusType_ENABLED, 19, tinkpb.OutputPrefixType_RAW),
	})
	gotPubKH, err := keyset.NewHandleWithNoSecrets(ks)
	if err != nil {
		t.Fatalf("NewHandleWithNoSecrets(%v) err = %v, want nil", ks, err)
	}

	plaintext := random.GetRandomBytes(200)
	ctxInfo := random.GetRandomBytes(100)

	// Encrypt with public key handle constructed from public key bytes.
	enc, err := hybrid.NewHybridEncrypt(gotPubKH)
	if err != nil {
		t.Fatalf("NewHybridEncrypt(%v) err = %v, want nil", gotPubKH, err)
	}
	ciphertext, err := enc.Encrypt(plaintext, ctxInfo)
	if err != nil {
		t.Fatalf("Encrypt(%x, %x) err = %v, want nil", plaintext, ctxInfo, err)
	}

	// Decrypt ciphertext with original private key handle.
	dec, err := hybrid.NewHybridDecrypt(privKH)
	if err != nil {
		t.Fatalf("NewHybridDecrypt(%v) err = %v, want nil", privKH, err)
	}
	gotPlaintext, err := dec.Decrypt(ciphertext, ctxInfo)
	if err != nil {
		t.Fatalf("Decrypt(%x, %x) err = %v, want nil", plaintext, ctxInfo, err)
	}
	if !bytes.Equal(gotPlaintext, plaintext) {
		t.Errorf("Decrypt(%x, %x) = %x, want %x", plaintext, ctxInfo, gotPlaintext, plaintext)
	}
}

func TestPublicKeyFromPrimaryKeyInvalidTemplateFails(t *testing.T) {
	keyTemplate := hybrid.DHKEM_X25519_HKDF_SHA256_HKDF_SHA256_CHACHA20_POLY1305_Raw_Key_Template()
	privKH, err := keyset.NewHandle(keyTemplate)
	if err != nil {
		t.Fatalf("NewHandle(%v) err = %v, want nil", keyTemplate, err)
	}
	pubKH, err := privKH.Public()
	if err != nil {
		t.Fatalf("Public() err = %v, want nil", err)
	}

	tests := []struct {
		name     string
		template *tinkpb.KeyTemplate
	}{
		{"AES_128_GCM", hybrid.DHKEM_X25519_HKDF_SHA256_HKDF_SHA256_AES_128_GCM_Key_Template()},
		{"AES_128_GCM_Raw", hybrid.DHKEM_X25519_HKDF_SHA256_HKDF_SHA256_AES_128_GCM_Raw_Key_Template()},
		{"AES_256_GCM", hybrid.DHKEM_X25519_HKDF_SHA256_HKDF_SHA256_AES_256_GCM_Key_Template()},
		{"AES_256_GCM_Raw", hybrid.DHKEM_X25519_HKDF_SHA256_HKDF_SHA256_AES_256_GCM_Raw_Key_Template()},
		{"CHACHA20_POLY1305", hybrid.DHKEM_X25519_HKDF_SHA256_HKDF_SHA256_CHACHA20_POLY1305_Key_Template()},
		{"invalid type URL", &tinkpb.KeyTemplate{
			TypeUrl:          "type.googleapis.com/google.crypto.tink.EciesAeadHkdfPrivateKey",
			Value:            keyTemplate.GetValue(),
			OutputPrefixType: tinkpb.OutputPrefixType_RAW,
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := subtle.PublicKeyFromPrimaryKey(pubKH, test.template); err == nil {
				t.Errorf("PublicKeyFromPrimaryKey(%v, %v) err = nil, want error", pubKH, test.template)
			}
		})
	}
}

func TestPublicKeyFromPrimaryKeyInvalidHandleFails(t *testing.T) {
	// Build valid key data.
	keyTemplate := hybrid.DHKEM_X25519_HKDF_SHA256_HKDF_SHA256_CHACHA20_POLY1305_Raw_Key_Template()
	privKH, err := keyset.NewHandle(keyTemplate)
	if err != nil {
		t.Fatalf("NewHandle(%v) err = %v, want nil", keyTemplate, err)
	}
	pubKH, err := privKH.Public()
	if err != nil {
		t.Fatalf("Public() err = %v, want nil", err)
	}
	pubKeyBytes, err := subtle.PublicKeyFromPrimaryKey(pubKH, keyTemplate)
	if err != nil {
		t.Fatalf("PublicKeyFromPrimaryKey(%v, %v) err = %v, want nil", pubKH, keyTemplate, err)
	}
	typeURL := "type.googleapis.com/google.crypto.tink.HpkePublicKey"
	validKD, err := keyDataFromBytes(t, pubKeyBytes, hpkepb.HpkeAead_CHACHA20_POLY1305, typeURL)
	if err != nil {
		t.Fatalf("keyDataFromBytes(%v, %v, %v) err = %v, want nil", pubKeyBytes, hpkepb.HpkeAead_CHACHA20_POLY1305, typeURL, err)
	}

	// Build key data with invalid type URL.
	invalidTypeURL := "type.googleapis.com/google.crypto.tink.EciesAeadHkdfPublicKey"
	invalidTypeURLKD, err := keyDataFromBytes(t, pubKeyBytes, hpkepb.HpkeAead_CHACHA20_POLY1305, invalidTypeURL)
	if err != nil {
		t.Fatalf("keyDataFromBytes(%v, %v, %v) err = %v, want nil", pubKeyBytes, hpkepb.HpkeAead_CHACHA20_POLY1305, invalidTypeURL, err)
	}

	// Build key data with invalid HPKE params.
	randomPubKeyBytes := random.GetRandomBytes(32)
	invalidAEAD := hpkepb.HpkeAead_AES_128_GCM
	invalidParamsKD, err := keyDataFromBytes(t, randomPubKeyBytes, invalidAEAD, typeURL)
	if err != nil {
		t.Fatalf("keyDataFromBytes(%v, %v, %v) err = %v, want nil", randomPubKeyBytes, invalidAEAD, typeURL, err)
	}

	tests := []struct {
		name         string
		primaryKeyID uint32
		key          *tinkpb.Keyset_Key
	}{
		{
			"empty",
			123,
			nil,
		},
		{
			"invalid status type",
			123,
			testutil.NewKey(validKD, tinkpb.KeyStatusType_DISABLED, 123, tinkpb.OutputPrefixType_RAW),
		},
		{
			"invalid prefix type",
			123,
			testutil.NewKey(validKD, tinkpb.KeyStatusType_ENABLED, 123, tinkpb.OutputPrefixType_TINK),
		},
		{
			"no primary key",
			0,
			testutil.NewKey(validKD, tinkpb.KeyStatusType_ENABLED, 123, tinkpb.OutputPrefixType_RAW),
		},
		{
			"invalid type URL",
			123,
			testutil.NewKey(invalidTypeURLKD, tinkpb.KeyStatusType_ENABLED, 123, tinkpb.OutputPrefixType_RAW),
		},
		{
			"invalid HPKE params",
			123,
			testutil.NewKey(invalidParamsKD, tinkpb.KeyStatusType_ENABLED, 123, tinkpb.OutputPrefixType_RAW),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ks := testutil.NewKeyset(test.primaryKeyID, []*tinkpb.Keyset_Key{test.key})
			handle, err := keyset.NewHandleWithNoSecrets(ks)
			if err != nil {
				t.Fatalf("NewHandleWithNoSecrets(%v) err = %v, want nil", ks, err)
			}
			if _, err := subtle.PublicKeyFromPrimaryKey(handle, keyTemplate); err == nil {
				t.Errorf("PublicKeyFromPrimaryKey(%v, %v) err = nil, want error", handle, keyTemplate)
			}
		})
	}
}

func keyDataFromBytes(t *testing.T, pubKeyBytes []byte, aeadID hpkepb.HpkeAead, typeURL string) (*tinkpb.KeyData, error) {
	t.Helper()

	pubKey := &hpkepb.HpkePublicKey{
		Version: 0,
		Params: &hpkepb.HpkeParams{
			Kem:  hpkepb.HpkeKem_DHKEM_X25519_HKDF_SHA256,
			Kdf:  hpkepb.HpkeKdf_HKDF_SHA256,
			Aead: aeadID,
		},
		PublicKey: pubKeyBytes,
	}

	serializedPubKey, err := proto.Marshal(pubKey)
	if err != nil {
		return nil, err
	}

	return testutil.NewKeyData(typeURL, serializedPubKey, tinkpb.KeyData_ASYMMETRIC_PUBLIC), nil
}
