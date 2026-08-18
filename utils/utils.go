package utils

import (
	"RMS/models"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	"github.com/sirupsen/logrus"
	"github.com/teris-io/shortid"
	"golang.org/x/crypto/bcrypt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var generator *shortid.Shortid

// validate is shared by every caller on purpose: the validator caches struct
// metadata internally, so building a new one per request throws that cache
// away. It is safe for concurrent use.
var validate = validator.New()

const generatorSeed = 1000

func init() {
	n, err := rand.Int(rand.Reader, big.NewInt(generatorSeed))
	if err != nil {
		logrus.Panicf("failed to initialize utilities with random seed, %+v", err)
		return
	}

	g, err := shortid.New(1, shortid.DefaultABC, n.Uint64())
	if err != nil {
		logrus.Panicf("Failed to initialize utils package with error: %+v", err)
	}

	generator = g
}

func newClientError(err error, statusCode int, messageToUser string, additionalInfoForDevs ...string) *models.ClientError {
	additionalInfoJoined := strings.Join(additionalInfoForDevs, "\n")
	if additionalInfoJoined == "" {
		additionalInfoJoined = messageToUser
	}
	errorID, _ := generator.Generate()
	var errString string
	if err != nil {
		errString = err.Error()
	}
	return &models.ClientError{
		ID:            errorID,
		MessageToUser: messageToUser,
		DeveloperInfo: additionalInfoJoined,
		Err:           errString,
		StatusCode:    statusCode,
		IsClientError: true,
	}
}

// ValidateStruct validates body against its `validate` tags and returns the
// validation error, leaving the response to the caller.
func ValidateStruct(body interface{}) error {
	return validate.Struct(body)
}

func ParseBody(body io.Reader, out interface{}) error {
	return json.NewDecoder(body).Decode(out)
}

func EncodeJSONBody(resp http.ResponseWriter, data interface{}) error {
	return json.NewEncoder(resp).Encode(data)
}

func RespondJSON(w http.ResponseWriter, statusCode int, body interface{}) {
	w.WriteHeader(statusCode)
	if body != nil {
		if err := EncodeJSONBody(w, body); err != nil {
			logrus.Errorf("Failed to respond JSON with error: %+v", err)
		}
	}
}

func RespondError(w http.ResponseWriter, statusCode int, err error, messageToUser string, additionalInfoForDevs ...string) {
	logrus.Errorf("status: %d, message: %s, err: %+v ", statusCode, messageToUser, err)
	clientError := newClientError(err, statusCode, messageToUser, additionalInfoForDevs...)
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(clientError); err != nil {
		logrus.Errorf("Failed to send error to caller with error: %+v", err)
	}
}

func HashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hashedPassword), err
}

func CheckPassword(password, hashedPassword string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

func GenerateJWT(userID, sessionID string, role models.Role) (string, error) {
	claims := jwt.MapClaims{
		"userId":    userID,
		"sessionId": sessionID,
		"role":      role,
		"exp":       time.Now().Add(time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(os.Getenv("JWT_SECRET_KEY")))
}

func SetupBindVars(stmt, bindVars string, length int) string {
	bindVars += ","
	stmt = fmt.Sprintf("%s %s", stmt, strings.Repeat(bindVars, length))
	return replaceSQL(strings.TrimSuffix(stmt, ","), "?")
}

func replaceSQL(old, searchPattern string) string {
	tmpCount := strings.Count(old, searchPattern)
	for m := 1; m <= tmpCount; m++ {
		old = strings.Replace(old, searchPattern, "$"+strconv.Itoa(m), 1)
	}
	return old
}
