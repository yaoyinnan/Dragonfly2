/*
 *     Copyright 2020 The Dragonfly Authors
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

package cdn

import "testing"

func Test_getBreakRange(t *testing.T) {
	type args struct {
		breakPoint       int64
		sourceFileLength int64
		taskRange        string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "breakPoint with range",
			args: args{
				breakPoint:       5,
				sourceFileLength: 100,
				taskRange:        "3-103",
			},
			want:    "8-103",
			wantErr: false,
		},
		{
			name: "range without breakPoint",
			args: args{
				breakPoint:       0,
				sourceFileLength: 100,
				taskRange:        "0-100",
			},
			want:    "0-100",
			wantErr: false,
		},
		{
			name: "",
			args: args{
				breakPoint:       100,
				sourceFileLength: 200,
				taskRange:        "100-300",
			},
			want:    "200-300",
			wantErr: false,
		},
		{
			name: "",
			args: args{
				breakPoint:       100,
				sourceFileLength: -1,
				taskRange:        "100-500",
			},
			want:    "100-",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getBreakRange(tt.args.breakPoint, tt.args.sourceFileLength, tt.args.taskRange)
			if (err != nil) != tt.wantErr {
				t.Errorf("getBreakRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getBreakRange() got = %v, want %v", got, tt.want)
			}
		})
	}
}
