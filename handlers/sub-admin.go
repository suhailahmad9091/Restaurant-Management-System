package handlers

import (
	"RMS/database/dbHelper"
	"RMS/middlewares"
	"RMS/models"
	"RMS/utils"
	"net/http"
)

func CreateSubAdmin(w http.ResponseWriter, r *http.Request) {
	var body models.SubAdminRequest

	userCtx := middlewares.UserContext(r)
	createdBy := userCtx.UserID
	role := models.RoleSubAdmin

	if parseErr := utils.ParseBody(r.Body, &body); parseErr != nil {
		utils.RespondError(w, http.StatusBadRequest, parseErr, "failed to parse request body")
		return
	}

	if valErr := utils.ValidateStruct(body); valErr != nil {
		utils.RespondError(w, http.StatusBadRequest, valErr, "input validation failed")
		return
	}

	exists, existsErr := dbHelper.IsUserExists(body.Email)
	if existsErr != nil {
		utils.RespondError(w, http.StatusInternalServerError, existsErr, "failed to check user existence")
		return
	}
	if exists {
		utils.RespondError(w, http.StatusConflict, nil, "sub-admin already exists")
		return
	}

	hashedPassword, hasErr := utils.HashPassword(body.Password)
	if hasErr != nil {
		utils.RespondError(w, http.StatusInternalServerError, hasErr, "failed to secure password")
		return
	}

	if saveErr := dbHelper.CreateSubAdmin(body.Name, body.Email, hashedPassword, createdBy, role); saveErr != nil {
		utils.RespondError(w, http.StatusInternalServerError, saveErr, "failed to create sub-admin")
		return
	}

	utils.RespondJSON(w, http.StatusCreated, struct {
		Message string `json:"message"`
	}{"sub-admin created successfully"})
}

func GetAllSubAdmins(w http.ResponseWriter, _ *http.Request) {
	subAdmins, getErr := dbHelper.GetAllSubAdmins()

	if getErr != nil {
		utils.RespondError(w, http.StatusInternalServerError, getErr, "failed to get sub-admin")
		return
	}

	utils.RespondJSON(w, http.StatusOK, subAdmins)
}
