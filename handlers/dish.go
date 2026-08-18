package handlers

import (
	"RMS/database/dbHelper"
	"RMS/middlewares"
	"RMS/models"
	"RMS/utils"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func CreateDish(w http.ResponseWriter, r *http.Request) {
	restaurantID := chi.URLParam(r, "restaurantId")
	var body models.CreateDishRequest

	if parseErr := utils.ParseBody(r.Body, &body); parseErr != nil {
		utils.RespondError(w, http.StatusBadRequest, parseErr, "failed to parse request body")
		return
	}

	if valErr := utils.ValidateStruct(body); valErr != nil {
		utils.RespondError(w, http.StatusBadRequest, valErr, "input validation failed")
		return
	}

	exists, existsErr := dbHelper.IsDishExists(body.Name, restaurantID)
	if existsErr != nil {
		utils.RespondError(w, http.StatusInternalServerError, existsErr, "failed to check dish existence")
		return
	}
	if exists {
		utils.RespondError(w, http.StatusConflict, nil, "dish already exists")
		return
	}

	if saveErr := dbHelper.CreateDish(body, restaurantID); saveErr != nil {
		utils.RespondError(w, http.StatusInternalServerError, saveErr, "failed to save dish")
		return
	}

	utils.RespondJSON(w, http.StatusCreated, struct {
		Message string `json:"message"`
	}{"dish created successfully"})
}

func GetAllDishes(w http.ResponseWriter, _ *http.Request) {
	dishes, getErr := dbHelper.GetAllDishes()

	if getErr != nil {
		utils.RespondError(w, http.StatusInternalServerError, getErr, "failed to get dishes")
		return
	}

	utils.RespondJSON(w, http.StatusOK, dishes)
}

func GetAllDishesBySubAdmin(w http.ResponseWriter, r *http.Request) {
	userCtx := middlewares.UserContext(r)
	loggedUserID := userCtx.UserID

	dishes, getErr := dbHelper.GetAllDishesBySubAdmin(loggedUserID)
	if getErr != nil {
		utils.RespondError(w, http.StatusInternalServerError, getErr, "failed to get dishes")
		return
	}

	utils.RespondJSON(w, http.StatusOK, dishes)
}

func DishesByRestaurant(w http.ResponseWriter, r *http.Request) {
	body := struct {
		RestaurantID string `json:"restaurantId" db:"restaurant_id" validate:"required"`
	}{}

	if parseErr := utils.ParseBody(r.Body, &body); parseErr != nil {
		utils.RespondError(w, http.StatusBadRequest, parseErr, "failed to parse request body")
		return
	}

	if valErr := utils.ValidateStruct(body); valErr != nil {
		utils.RespondError(w, http.StatusBadRequest, valErr, "input validation failed")
		return
	}

	dishes, getErr := dbHelper.DishesByRestaurant(body.RestaurantID)
	if getErr != nil {
		utils.RespondError(w, http.StatusInternalServerError, getErr, "failed to get dishes")
		return
	}

	utils.RespondJSON(w, http.StatusOK, dishes)
}
