package handlers

import (
	"RMS/database"
	"RMS/database/dbHelper"
	"RMS/middlewares"
	"RMS/models"
	"RMS/utils"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/jmoiron/sqlx"
	"golang.org/x/sync/errgroup"
)

func CreateUser(w http.ResponseWriter, r *http.Request) {
	var body models.UserRequest

	userCtx := middlewares.UserContext(r)
	createdBy := userCtx.UserID
	role := models.RoleUser

	if parseErr := utils.ParseBody(r.Body, &body); parseErr != nil {
		utils.RespondError(w, http.StatusBadRequest, parseErr, "failed to parse request body")
		return
	}

	v := validator.New()
	if err := v.Struct(body); err != nil {
		utils.RespondError(w, http.StatusBadRequest, err, "input validation failed")
		return
	}

	exists, existsErr := dbHelper.IsUserExists(body.Email)
	if existsErr != nil {
		utils.RespondError(w, http.StatusInternalServerError, existsErr, "failed to check user existence")
		return
	}
	if exists {
		utils.RespondError(w, http.StatusConflict, nil, "user already exists")
		return
	}

	hashedPassword, hasErr := utils.HashPassword(body.Password)
	if hasErr != nil {
		utils.RespondError(w, http.StatusInternalServerError, hasErr, "failed to secure password")
		return
	}

	if txErr := database.Tx(func(tx *sqlx.Tx) error {
		userId, saveErr := dbHelper.CreateUser(tx, body.Name, body.Email, hashedPassword, createdBy, role)
		if saveErr != nil {
			return saveErr
		}
		return dbHelper.CreateUserAddress(tx, userId, body.Address)
	}); txErr != nil {
		utils.RespondError(w, http.StatusInternalServerError, txErr, "failed to create user")
		return
	}

	utils.RespondJSON(w, http.StatusCreated, struct {
		Message string `json:"message"`
	}{"user created successfully"})
}

func LoginUser(w http.ResponseWriter, r *http.Request) {
	var body models.LoginRequest

	if parseErr := utils.ParseBody(r.Body, &body); parseErr != nil {
		utils.RespondError(w, http.StatusBadRequest, parseErr, "failed to parse request body")
		return
	}

	v := validator.New()
	if err := v.Struct(body); err != nil {
		utils.RespondError(w, http.StatusBadRequest, err, "input validation failed")
		return
	}

	userID, role, userErr := dbHelper.GetUserInfo(body)
	if userErr != nil {
		utils.RespondError(w, http.StatusInternalServerError, userErr, "failed to find user")
		return
	}

	if userID == "" || role == "" {
		utils.RespondError(w, http.StatusOK, nil, "user not found")
		return
	}

	sessionID, crtErr := dbHelper.CreateUserSession(userID)
	if crtErr != nil {
		utils.RespondError(w, http.StatusInternalServerError, crtErr, "failed to create user session")
		return
	}

	token, genErr := utils.GenerateJWT(userID, sessionID, role)
	if genErr != nil {
		utils.RespondError(w, http.StatusInternalServerError, genErr, "failed to generate token")
		return
	}

	utils.RespondJSON(w, http.StatusOK, struct {
		Message string `json:"message"`
		Token   string `json:"token"`
	}{"login successful", token})
}

func LogoutUser(w http.ResponseWriter, r *http.Request) {
	userCtx := middlewares.UserContext(r)
	sessionID := userCtx.SessionID

	if delErr := dbHelper.DeleteUserSession(sessionID); delErr != nil {
		utils.RespondError(w, http.StatusInternalServerError, delErr, "failed to delete user session")
		return
	}

	utils.RespondJSON(w, http.StatusOK, struct {
		Message string `json:"message"`
	}{"logout successful"})
}

func GetAllUsersByAdmin(w http.ResponseWriter, _ *http.Request) {
	users, getErr := dbHelper.GetAllUsersByAdmin()

	if getErr != nil {
		utils.RespondError(w, http.StatusInternalServerError, getErr, "failed to get users")
		return
	}

	utils.RespondJSON(w, http.StatusOK, users)
}

func GetAllUsersBySubAdmin(w http.ResponseWriter, r *http.Request) {
	userCtx := middlewares.UserContext(r)
	loggedUserID := userCtx.UserID

	users, getErr := dbHelper.GetAllUsersBySubAdmin(loggedUserID)
	if getErr != nil {
		utils.RespondError(w, http.StatusInternalServerError, getErr, "failed to get users")
		return
	}

	utils.RespondJSON(w, http.StatusOK, users)
}

func CalculateDistance(w http.ResponseWriter, r *http.Request) {
	var body models.DistanceRequest

	if parseErr := utils.ParseBody(r.Body, &body); parseErr != nil {
		utils.RespondError(w, http.StatusBadRequest, parseErr, "failed to parse request body")
		return
	}

	v := validator.New()
	if err := v.Struct(body); err != nil {
		utils.RespondError(w, http.StatusBadRequest, err, "input validation failed")
		return
	}

	var eg errgroup.Group
	var userCoordinates, restaurantCoordinates models.Coordinates

	// Each goroutine keeps its own error. Sharing one `err` variable let both
	// write it concurrently, which is a data race and could also hand the
	// caller the wrong failure.
	eg.Go(func() error {
		var getErr error
		userCoordinates, getErr = dbHelper.GetUserCoordinates(body.UserAddressID)
		return getErr
	})

	eg.Go(func() error {
		var getErr error
		restaurantCoordinates, getErr = dbHelper.GetRestaurantCoordinates(body.RestaurantAddressID)
		return getErr
	})

	ergErr := eg.Wait()
	if ergErr != nil {
		utils.RespondError(w, http.StatusInternalServerError, ergErr, "failed to get coordinates")
		return
	}

	distance, calErr := dbHelper.CalculateDistance(userCoordinates, restaurantCoordinates)
	if calErr != nil {
		utils.RespondError(w, http.StatusInternalServerError, calErr, "failed to calculate distance")
		return
	}

	utils.RespondJSON(w, http.StatusOK, struct {
		Distance float64 `json:"distance"`
	}{distance})
}
