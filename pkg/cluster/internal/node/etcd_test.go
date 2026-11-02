/*
Copyright 2025 The Kubernetes Authors.

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

package node

import (
	"testing"
)

func TestParseEtcdMemberLine(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		line        string
		wantID      string
		wantName    string
		wantLearner bool
		wantErr     bool
	}{
		{
			name:        "valid member line",
			line:        "e58c878e0e01014, started, kind-control-plane, https://172.18.0.2:2380, https://172.18.0.2:2379, false",
			wantID:      "e58c878e0e01014",
			wantName:    "kind-control-plane",
			wantLearner: false,
		},
		{
			name:        "valid learner line",
			line:        "abc123def456789, started, kind-worker, https://172.18.0.3:2380, https://172.18.0.3:2379, true",
			wantID:      "abc123def456789",
			wantName:    "kind-worker",
			wantLearner: true,
		},
		{
			name:        "leading/trailing whitespace",
			line:        "  abc1234 , started , my-node , https://10.0.0.1:2380 , https://10.0.0.1:2379 , false  ",
			wantID:      "abc1234",
			wantName:    "my-node",
			wantLearner: false,
		},
		{
			name:    "empty line",
			line:    "",
			wantErr: true,
		},
		{
			name:    "too few fields",
			line:    "id, started, name",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseEtcdMemberLine(tc.line)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for line %q, got none", tc.line)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.ID != tc.wantID {
				t.Errorf("ID: got %q, want %q", got.ID, tc.wantID)
			}
			if got.Name != tc.wantName {
				t.Errorf("Name: got %q, want %q", got.Name, tc.wantName)
			}
			if got.IsLearner != tc.wantLearner {
				t.Errorf("IsLearner: got %v, want %v", got.IsLearner, tc.wantLearner)
			}
		})
	}
}
