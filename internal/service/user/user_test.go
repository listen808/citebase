package user

import (
	"crypto/md5"
	"fmt"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestIsMD5Hash(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "valid lowercase MD5 hash",
			input: "d41d8cd98f00b204e9800998ecf8427e",
			want:  true,
		},
		{
			name:  "valid uppercase MD5 hash",
			input: "D41D8CD98F00B204E9800998ECF8427E",
			want:  true,
		},
		{
			name:  "valid mixed case MD5 hash",
			input: "D41d8Cd98f00B204e9800998Ecf8427e",
			want:  true,
		},
		{
			name:  "too short",
			input: "d41d8cd98f00b204e9800998ecf8427",
			want:  false,
		},
		{
			name:  "too long",
			input: "d41d8cd98f00b204e9800998ecf8427e0",
			want:  false,
		},
		{
			name:  "empty string",
			input: "",
			want:  false,
		},
		{
			name:  "32 chars but contains non-hex character",
			input: "d41d8cd98f00b204e9800998ecf8427g",
			want:  false,
		},
		{
			name:  "bcrypt hash prefix",
			input: "$2a$10$N9qo8uLOickgx2ZMRZoMye",
			want:  false,
		},
		{
			name:  "plain text password",
			input: "password123",
			want:  false,
		},
		{
			name:  "all zeros valid hex",
			input: "00000000000000000000000000000000",
			want:  true,
		},
		{
			name:  "all f valid hex",
			input: "ffffffffffffffffffffffffffffffff",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isMD5Hash(tt.input)
			if got != tt.want {
				t.Errorf("isMD5Hash(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestVerifyPassword(t *testing.T) {
	// Pre-compute test passwords
	plainPassword := "testPassword123!"
	wrongPassword := "wrongPassword456"

	// Generate bcrypt hash
	bcryptHash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to generate bcrypt hash: %v", err)
	}

	// Generate MD5 hash
	md5Hash := fmt.Sprintf("%x", md5.Sum([]byte(plainPassword)))

	tests := []struct {
		name           string
		hashedPassword string
		password       string
		want           bool
	}{
		{
			name:           "correct bcrypt password",
			hashedPassword: string(bcryptHash),
			password:       plainPassword,
			want:           true,
		},
		{
			name:           "wrong bcrypt password",
			hashedPassword: string(bcryptHash),
			password:       wrongPassword,
			want:           false,
		},
		{
			name:           "correct MD5 password",
			hashedPassword: md5Hash,
			password:       plainPassword,
			want:           true,
		},
		{
			name:           "wrong MD5 password",
			hashedPassword: md5Hash,
			password:       wrongPassword,
			want:           false,
		},
		{
			name:           "plain text stored password always fails",
			hashedPassword: "plaintext",
			password:       "plaintext",
			want:           false,
		},
		{
			name:           "empty hashed password",
			hashedPassword: "",
			password:       "anything",
			want:           false,
		},
		{
			name:           "empty input password with bcrypt hash",
			hashedPassword: string(bcryptHash),
			password:       "",
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := verifyPassword(tt.hashedPassword, tt.password)
			if got != tt.want {
				t.Errorf("verifyPassword() = %v, want %v", got, tt.want)
			}
		})
	}
}
