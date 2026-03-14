package utils

import (
	"testing"
)

func TestGenerateLambdaARN(t *testing.T) {
	tests := []struct {
		name         string
		region       string
		accountID    string
		functionName string
		want         string
	}{
		{
			name:         "Standard ARN",
			region:       "us-east-1",
			accountID:    "123456789012",
			functionName: "my-function",
			want:         "arn:serw:lambda:us-east-1:123456789012:function/my-function",
		},
		{
			name:         "Another Region",
			region:       "eu-west-1",
			accountID:    "999999999999",
			functionName: "another-func",
			want:         "arn:serw:lambda:eu-west-1:999999999999:function/another-func",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GenerateLambdaARN(tt.region, tt.accountID, tt.functionName); got != tt.want {
				t.Errorf("GenerateLambdaARN() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseLambdaARN(t *testing.T) {
	tests := []struct {
		name    string
		arnStr  string
		want    *LambdaARN
		wantErr bool
	}{
		{
			name:   "Valid ARN",
			arnStr: "arn:serw:lambda:us-east-1:123456789012:function/my-func",
			want: &LambdaARN{
				Partition:    "serw",
				Service:      "lambda",
				Region:       "us-east-1",
				AccountID:    "123456789012",
				ResourceType: "function",
				ResourceID:   "my-func",
			},
			wantErr: false,
		},
		{
			name:    "Invalid Parts Count",
			arnStr:  "arn:serw:lambda:us-east-1:123456789012",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid Prefix",
			arnStr:  "notarn:serw:lambda:us-east-1:123456789012:function/my-func",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid Partition",
			arnStr:  "arn:aws:lambda:us-east-1:123456789012:function/my-func",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid Service",
			arnStr:  "arn:serw:s3:us-east-1:123456789012:function/my-func",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid Resource Format",
			arnStr:  "arn:serw:lambda:us-east-1:123456789012:bucket/my-bucket",
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLambdaARN(tt.arnStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLambdaARN() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if *got != *tt.want {
					t.Errorf("ParseLambdaARN() got = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestValidateLambdaARN(t *testing.T) {
	tests := []struct {
		name    string
		arnStr  string
		wantErr bool
	}{
		{"Valid", "arn:serw:lambda:us-east-1:123456789012:function/my-func", false},
		{"Invalid", "arn:aws:lambda:us-east-1:123456789012:function/my-func", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateLambdaARN(tt.arnStr); (err != nil) != tt.wantErr {
				t.Errorf("ValidateLambdaARN() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLambdaARN_String(t *testing.T) {
	arn := &LambdaARN{
		Partition:    "serw",
		Service:      "lambda",
		Region:       "us-east-1",
		AccountID:    "123456789012",
		ResourceType: "function",
		ResourceID:   "my-func",
	}
	want := "arn:serw:lambda:us-east-1:123456789012:function/my-func"
	if got := arn.String(); got != want {
		t.Errorf("LambdaARN.String() = %v, want %v", got, want)
	}
}
