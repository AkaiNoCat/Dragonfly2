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

package middlewares

import (
	"net/http"

	"d7y.io/dragonfly/v2/internal/dfcodes"
	"d7y.io/dragonfly/v2/internal/dferrors"
	"github.com/VividCortex/mysqlerr"
	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	redigo "github.com/gomodule/redigo/redis"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type ErrorResponse struct {
	Message     string `json:"message,omitempty"`
	Error       string `json:"errors,omitempty"`
	DocumentURL string `json:"documentation_url,omitempty"`
}

func Error() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		err := c.Errors.Last()
		if err == nil {
			return
		}

		// Redigo error handler
		if errors.Is(err, redigo.ErrNil) {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Message: http.StatusText(http.StatusNotFound),
			})
			c.Abort()
			return
		}

		// RPC error handler
		if err, ok := errors.Cause(err.Err).(*dferrors.DfError); ok {
			switch err.Code {
			case dfcodes.InvalidResourceType:
				c.JSON(http.StatusBadRequest, ErrorResponse{
					Message: http.StatusText(http.StatusBadRequest),
				})
				c.Abort()
				return
			default:
				c.JSON(http.StatusInternalServerError, ErrorResponse{
					Message: http.StatusText(http.StatusInternalServerError),
				})
				c.Abort()
				return
			}
		}

		// Bcrypt error handler
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			c.JSON(http.StatusUnauthorized, ErrorResponse{
				Message: http.StatusText(http.StatusUnauthorized),
			})
			c.Abort()
			return
		}

		// GORM error handler
		if errors.Is(err.Err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Message: http.StatusText(http.StatusNotFound),
			})
			c.Abort()
			return
		}

		// Mysql error handler
		if err, ok := errors.Cause(err.Err).(*mysql.MySQLError); ok {
			switch err.Number {
			case mysqlerr.ER_DUP_ENTRY:
				c.JSON(http.StatusConflict, ErrorResponse{
					Message: http.StatusText(http.StatusConflict),
				})
				c.Abort()
				return
			default:
				c.JSON(http.StatusInternalServerError, ErrorResponse{
					Message: http.StatusText(http.StatusInternalServerError),
				})
				c.Abort()
				return
			}
		}

		// Unknown error
		c.JSON(http.StatusInternalServerError, nil)
	}
}
