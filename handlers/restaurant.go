package handlers

import (
	"RMS/database/dbHelper"
	"RMS/middlewares"
	"RMS/models"
	"RMS/utils"
	"net/http"
)

func CreateRestaurant(w http.ResponseWriter, r *http.Request) {
	var body models.CreateRestaurantRequest

	userCtx := middlewares.UserContext(r)
	createdBy := userCtx.UserID

	if parseErr := utils.ParseBody(r.Body, &body); parseErr != nil {
		utils.RespondError(w, http.StatusBadRequest, parseErr, "failed to parse request body")
		return
	}

	if valErr := utils.ValidateStruct(body); valErr != nil {
		utils.RespondError(w, http.StatusBadRequest, valErr, "input validation failed")
		return
	}

	exists, existsErr := dbHelper.IsRestaurantExists(body.Name, body.Address)
	if existsErr != nil {
		utils.RespondError(w, http.StatusInternalServerError, existsErr, "failed to check restaurant existence")
		return
	}
	if exists {
		utils.RespondError(w, http.StatusConflict, nil, "restaurant already exists")
		return
	}

	if saveErr := dbHelper.CreateRestaurant(body, createdBy); saveErr != nil {
		utils.RespondError(w, http.StatusInternalServerError, saveErr, "failed to save restaurant")
		return
	}

	utils.RespondJSON(w, http.StatusCreated, struct {
		Message string `json:"message"`
	}{"restaurant created successfully"})
}

func GetAllRestaurants(w http.ResponseWriter, _ *http.Request) {
	restaurants, getErr := dbHelper.GetAllRestaurants()

	if getErr != nil {
		utils.RespondError(w, http.StatusInternalServerError, getErr, "failed to get restaurants")
		return
	}

	utils.RespondJSON(w, http.StatusOK, restaurants)
}

func GetAllRestaurantsBySubAdmin(w http.ResponseWriter, r *http.Request) {
	userCtx := middlewares.UserContext(r)
	loggedUserID := userCtx.UserID

	restaurants, getErr := dbHelper.GetAllRestaurantsBySubAdmin(loggedUserID)
	if getErr != nil {
		utils.RespondError(w, http.StatusInternalServerError, getErr, "failed to get restaurants")
		return
	}

	utils.RespondJSON(w, http.StatusOK, restaurants)
}
