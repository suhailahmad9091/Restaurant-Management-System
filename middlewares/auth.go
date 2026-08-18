package middlewares

import (
	"RMS/database/dbHelper"
	"RMS/models"
	"RMS/utils"
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"

	"github.com/golang-jwt/jwt/v5"
)

type ContextKeys string

const (
	userContext ContextKeys = "userContext"
)

func Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := r.Header.Get("token")
		if tokenString == "" {
			utils.RespondError(w, http.StatusUnauthorized, nil, "token header missing")
			return
		}

		token, parseErr := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("invalid signing method")
			}
			return []byte(os.Getenv("JWT_SECRET_KEY")), nil
		})

		if parseErr != nil || !token.Valid {
			utils.RespondError(w, http.StatusUnauthorized, parseErr, "invalid token")
			return
		}

		claimValues, ok := token.Claims.(jwt.MapClaims)
		if !ok || !token.Valid {
			utils.RespondError(w, http.StatusUnauthorized, nil, "invalid token claims")
			return
		}

		// A token signed with our key but carrying the wrong claim shapes is a
		// bad token, not a server fault. Asserting these unchecked would panic
		// into the recovery middleware and report 500.
		userID, userIDOk := claimValues["userId"].(string)
		sessionID, sessionIDOk := claimValues["sessionId"].(string)
		role, roleOk := claimValues["role"].(string)
		if !userIDOk || !sessionIDOk || !roleOk {
			utils.RespondError(w, http.StatusUnauthorized, nil, "invalid token claims")
			return
		}

		archivedAt, err := dbHelper.GetArchivedAt(sessionID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				utils.RespondError(w, http.StatusUnauthorized, err, "invalid token")
				return
			}
			utils.RespondError(w, http.StatusInternalServerError, err, "internal server error")
			return
		}

		if archivedAt != nil {
			utils.RespondError(w, http.StatusUnauthorized, nil, "invalid token")
			return
		}

		user := &models.UserCtx{
			UserID:    userID,
			SessionID: sessionID,
			Role:      models.Role(role),
		}

		ctx := context.WithValue(r.Context(), userContext, user)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

func UserContext(r *http.Request) *models.UserCtx {
	user, _ := r.Context().Value(userContext).(*models.UserCtx)
	return user
}
