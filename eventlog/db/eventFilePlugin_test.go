// RAINBOND, Application Management Platform
// Copyright (C) 2014-2017 Goodrain Co., Ltd.

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package db

import (
	"testing"
	"time"
)

func TestEventFileSaveMessage(t *testing.T) {
	eventFilePlugin := EventFilePlugin{
		HomePath: "./test",
	}
	if err := eventFilePlugin.SaveMessage([]*EventLogMessage{
		&EventLogMessage{
			EventID: "eventidsadasd",
			Level:   "info",
			Message: "1ajdsnadskfasndjn afnasdfnln asdfjnajksndfjk",
			Time:    time.Now().Format(time.RFC3339),
		},
		&EventLogMessage{
			EventID: "eventidsadasd",
			Level:   "debug",
			Message: "2ajdsnadskfasndjn afnasdfnln asdfjnajksndfjk",
			Time:    time.Now().Format(time.RFC3339),
		},
		&EventLogMessage{
			EventID: "eventidsadasd",
			Level:   "error",
			Message: "3ajdsnadskfasndjn afnasdfnln asdfjnajksndfjk",
			Time:    time.Now().Format(time.RFC3339),
		},
		&EventLogMessage{
			EventID: "eventidsadasd",
			Level:   "debug",
			Message: "4ajdsnadskfasndjn afnasdfnln asdfjnajksndfjk",
			Time:    time.Now().Add(time.Hour).Format(time.RFC3339),
		},
		&EventLogMessage{
			EventID: "eventidsadasd",
			Level:   "info",
			Message: "5ajdsnadskfasndjn afnasdfnln asdfjnajksndfjk",
			Time:    time.Now().Format(time.RFC3339),
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := eventFilePlugin.SaveMessage([]*EventLogMessage{
		&EventLogMessage{
			EventID: "eventidsadasd",
			Level:   "info",
			Message: "1ajdsnadskfasndjn afnasdfnln asdfjnajksndfjk2",
			Time:    time.Now().Format(time.RFC3339),
		},
		&EventLogMessage{
			EventID: "eventidsadasd",
			Level:   "debug",
			Message: "2ajdsnadskfasndjn afnasdfnln asdfjnajksndfjk2",
			Time:    time.Now().Format(time.RFC3339),
		},
		&EventLogMessage{
			EventID: "eventidsadasd",
			Level:   "error",
			Message: "3ajdsnadskfasndjn afnasdfnln asdfjnajksndfjk2",
			Time:    time.Now().Format(time.RFC3339),
		},
		&EventLogMessage{
			EventID: "eventidsadasd",
			Level:   "debug",
			Message: "4ajdsnadskfasndjn afnasdfnln asdfjnajksndfjk2",
			Time:    time.Now().Add(time.Hour).Format(time.RFC3339),
		},
		&EventLogMessage{
			EventID: "eventidsadasd",
			Level:   "info",
			Message: "5ajdsnadskfasndjn afnasdfnln asdfjnajksndfjk2",
			Time:    time.Now().Format(time.RFC3339),
		},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestGetMessage(t *testing.T) {
	eventFilePlugin := EventFilePlugin{
		HomePath: "/tmp",
	}
	list, err := eventFilePlugin.GetMessages("eventidsadasd", "debug")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(list)
}
