//  Copyright (c) 2012 Couchbase, Inc.
//  Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file
//  except in compliance with the License. You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software distributed under the
//  License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
//  either express or implied. See the License for the specific language governing permissions
//  and limitations under the License.

package auth

import (
	"fmt"
	"net/http"
	"regexp"

	"github.com/dchest/passwordhash"

	"github.com/couchbaselabs/basecouch/base"
	ch "github.com/couchbaselabs/basecouch/channels"
)

/** Persistent information about a user. */
type User struct {
	Name         string                     `json:"name,omitempty"`
	Email        string                     `json:"email,omitempty"`
	PasswordHash *passwordhash.PasswordHash `json:"passwordhash,omitempty"`
	Channels     []string                   `json:"channels"`

	Password	 *string					`json:"password,omitempty"`
}

var kValidEmailRegexp *regexp.Regexp

func init() {
	var err error
	kValidEmailRegexp, err = regexp.Compile(`^[-.\w]+@\w[-.\w]+$`)
	if err != nil {panic("Bad IsValidEmail regexp")}
}

func IsValidEmail(email string) bool {
	return kValidEmailRegexp.MatchString(email)
}

// Creates a new User object.
func NewUser(username string, password string, channels []string) (*User, error) {
	user := &User{Name: username, Channels: ch.SimplifyChannels(channels, true)}
	user.SetPassword(password)
	if err := user.Validate(); err != nil {
		return nil, err
	}
	return user, nil
}

// Checks whether this User object contains valid data; if not, returns an error.
func (user *User) Validate() error {
	if match, _ := regexp.MatchString(`^\w*$`, user.Name); !match {
		return &base.HTTPError{http.StatusBadRequest, fmt.Sprintf("Invalid username %q", user.Name)}
	} else if (user.Name == "") != (user.PasswordHash == nil) {
		return &base.HTTPError{http.StatusBadRequest, "Invalid password"}
	} else if user.Email != "" && !IsValidEmail(user.Email) {
		return &base.HTTPError{http.StatusBadRequest, "Invalid email address"}
	}
	return nil
}

// Returns true if the given password is correct for this user.
func (user *User) Authenticate(password string) bool {
	if user == nil {
		return false
	} else if user.PasswordHash == nil {
		if password != "" {
			return false
		}
	} else if !user.PasswordHash.EqualToPassword(password) {
		return false
	}
	return true
}

// Changes a user's password to the given string.
func (user *User) SetPassword(password string) {
	if password == "" {
		user.PasswordHash = nil
	} else {
		user.PasswordHash = passwordhash.New(password)
	}
}


//////// USER CHANNEL AUTHORIZATION:


func (user *User) UnauthError(message string) error {
	if user.Name == "" {
		return &base.HTTPError{http.StatusUnauthorized, "login required"}
	}
	return &base.HTTPError{http.StatusForbidden, message}
}

// Returns true if the User is allowed to access the channel.
// A nil User means access control is disabled, so the function will return true.
func (user *User) CanSeeChannel(channel string) bool {
	return user == nil || channel == "*" || ch.ContainsChannel(user.Channels, channel) ||
		ch.ContainsChannel(user.Channels, "*")
}

// Returns true if the User is allowed to access all of the given channels.
// A nil User means access control is disabled, so the function will return true.
func (user *User) CanSeeAllChannels(channels []string) bool {
	if channels != nil {
		for _, channel := range channels {
			if !user.CanSeeChannel(channel) {
				return false
			}
		}
	}
	return true
}

// Returns true if the User is allowed to access any of the given channels.
// A nil User means access control is disabled, so the function will return true.
func (user *User) CanSeeAnyChannels(channels []string) bool {
	if channels != nil {
		for _, channel := range channels {
			if user.CanSeeChannel(channel) {
				return true
			}
		}
	}
	// If user has wildcard access, allow it anyway
	return ch.ContainsChannel(user.Channels, "*")
}

// Returns an HTTP 403 error if the User is not allowed to access all the given channels.
// A nil User means access control is disabled, so the function will return nil.
func (user *User) AuthorizeAllChannels(channels []string) error {
	var forbidden []string
	for _, channel := range channels {
		if !user.CanSeeChannel(channel) {
			if forbidden == nil {
				forbidden = make([]string, 0, len(channels))
			}
			forbidden = append(forbidden, channel)
		}
	}
	if forbidden != nil {
		return user.UnauthError(fmt.Sprintf("You are not allowed to see channels %v", forbidden))
	}
	return nil
}

// Returns an HTTP 403 error if the User is not allowed to access any of the given channels.
// A nil User means access control is disabled, so the function will return nil.
func (user *User) AuthorizeAnyChannels(channels []string) error {
	if !user.CanSeeAnyChannels(channels) {
		return user.UnauthError("You are not allowed to see this")
	}
	return nil
}