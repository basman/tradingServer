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
	"tradingServer/storage"
)

func (s *server) DbTransaction() gin.HandlerFunc {
	return func(c *gin.Context) {
		tx, err := s.db.Begin()
		if err != nil {
			log.Printf("begin transaction failed: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.Next()

		err = tx.Commit()
		if err != nil {
			log.Printf("commit transaction failed: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
	}
}

func (s *server) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")

		login, pw, err := decodeAuthHeader(auth)
		if err != nil {
			log.Printf("authorization failed: %v", err)
			c.Header("WWW-Authenticate","Basic realm=\"Hail to the king!\"")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		acc, err := s.db.GetAccount(login)
		if err != nil {
			log.Printf("login query failed: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		if acc == nil {
			c.Header("WWW-Authenticate","Basic realm=\"Hail to the king!\"")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		pwHashed := storage.HashEncodePassword(pw)
		if pwHashed != acc.Password {
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
