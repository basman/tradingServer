package server

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"strings"
	"time"
	"tradingServer/storage"
)

func (s *server) dbTransaction() gin.HandlerFunc {
	return func(c *gin.Context) {
		tx, err := s.db.Begin()
		if err != nil {
			log.Printf("begin transaction failed: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.Next()

		if c.IsAborted() {
			tx.Rollback()
			return
		}

		err = tx.Commit()
		if err != nil {
			log.Printf("commit transaction failed: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
	}
}

func (s *server) authRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")

		login, pw, err := decodeAuthHeader(auth)
		if err != nil {
			log.Printf("authorization failed: %v", err)
			c.Header("WWW-Authenticate", "Basic realm=\"Hail to the king!\"")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		acc, err := s.db.GetAccount(login)
		if err != nil {
			log.Printf("login query failed: %v", err)
			c.Header("WWW-Authenticate","Basic realm=\"Hail to the king!\"")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		if acc == nil {
			log.Printf("authorization failed: login '%v' unknown", login)
			c.Header("WWW-Authenticate","Basic realm=\"Hail to the king!\"")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		if !acc.VerifyPassword(pw) {
			log.Printf("authorization failed: password mismatch for login '%v': got '%v'", login, pw)
			c.Header("WWW-Authenticate","Basic realm=\"Hail to the king!\"")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c.Set("login", acc.Login)
		c.Set("balance", acc.Balance)

		c.Next()
	}
}

func decodeAuthHeader(authHeader string) (string, string, error) {
	if authHeader == "" {
		return "", "", errors.New("no authorization header")
	}

	idxSpc := strings.IndexByte(authHeader, ' ')
	if idxSpc <= 1 {
		return "", "", errors.New("no space separator found in authorization header")
	}

	method := authHeader[:idxSpc]
	if strings.ToUpper(method) != "BASIC" {
		return "", "", fmt.Errorf("unknown authorization method '%v'", method)
	}

	token := authHeader[idxSpc+1:]

	buf, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return "", "", err
	}

	idxSep := bytes.IndexByte(buf, ':')
	if idxSep <= 1 {
		return "", "", errors.New("no : separator found")
	}

	login := string(buf[:idxSep])
	pw := string(buf[idxSep+1:])

	return login, pw, nil
}

func (s *server) accessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		duration := time.Now().Sub(start)

		login, ok := c.Get("login")
		if !ok {
			login = ""
		}

		err := s.db.LogAccess(storage.AccessLogEntry{
			Duration:      float64(duration.Microseconds())/1000000,
			Login:         login.(string),
			Path:          c.FullPath(),
			RemoteAddress: c.Request.RemoteAddr,
			StatusCode:    c.Writer.Status(),
			Time:          start,
		})

		if err != nil {
			log.Printf("%v", err)
		}
	}
}

func (s *server) rateLimit(group string, reqPerSec float64) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := group
		// apply rate limit per user if request is authenticated
		loginRaw, ok := c.Get("login")
		if ok {
			login := loginRaw.(string)
			id = group + "-" + login
		}

		if s.rateLimitState.CheckAndUpdate(id, reqPerSec) {
			c.Next()
		} else {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, newUserError("rate limit of %v requests per second exceeded", reqPerSec))
		}
	}
}
