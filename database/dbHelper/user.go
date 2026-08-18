package dbHelper

import (
	"RMS/database"
	"RMS/models"
	"RMS/utils"
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
)

func IsUserExists(email string) (bool, error) {
	SQL := `SELECT count(id) > 0 as is_exist
			  FROM users
			  WHERE email = TRIM($1)
			    AND archived_at IS NULL`

	var check bool
	chkErr := database.RMS.Get(&check, SQL, email)
	return check, chkErr
}

func CreateUser(tx *sqlx.Tx, name, email, password, createdBy string, role models.Role) (string, error) {
	SQL := `INSERT INTO users (name, email, password, created_by, role)
			  VALUES (TRIM($1), TRIM($2), $3, $4, $5) RETURNING id`

	var userID string
	crtErr := tx.Get(&userID, SQL, name, email, password, createdBy, role)
	return userID, crtErr
}

func CreateUserAddress(tx *sqlx.Tx, userID string, addresses []models.AddressRequest) error {
	SQL := `INSERT INTO address (user_id, address, latitude, longitude) VALUES`

	values := make([]interface{}, 0)
	for i := range addresses {
		values = append(values,
			userID,
			addresses[i].Address,
			addresses[i].Latitude,
			addresses[i].Longitude,
		)
	}
	SQL = utils.SetupBindVars(SQL, "(?, ?, ?, ?)", len(addresses))

	_, err := tx.Exec(SQL, values...)
	return err
}

func CreateUserSession(userID string) (string, error) {
	var sessionID string
	SQL := `INSERT INTO user_session(user_id) 
              VALUES ($1) RETURNING id`
	crtErr := database.RMS.Get(&sessionID, SQL, userID)
	return sessionID, crtErr
}

// ErrInvalidCredentials is returned when the email is unknown or the password
// does not match. Both cases share one error on purpose, so callers cannot
// turn the login endpoint into a way of discovering registered emails.
var ErrInvalidCredentials = errors.New("invalid credentials")

func GetUserInfo(body models.LoginRequest) (string, models.Role, error) {
	SQL := `SELECT u.id,
       			   u.role,
       			   u.password
			  FROM users u
			  WHERE u.email = TRIM($1)
			    AND u.archived_at IS NULL`

	var user models.LoginData
	if getErr := database.RMS.Get(&user, SQL, body.Email); getErr != nil {
		if errors.Is(getErr, sql.ErrNoRows) {
			return "", "", ErrInvalidCredentials
		}
		return "", "", getErr
	}
	if passwordErr := utils.CheckPassword(body.Password, user.PasswordHash); passwordErr != nil {
		return "", "", ErrInvalidCredentials
	}
	return user.ID, user.Role, nil
}

func GetArchivedAt(sessionID string) (*time.Time, error) {
	var archivedAt *time.Time

	SQL := `SELECT archived_at 
              FROM user_session 
              WHERE id = $1
              	AND archived_at IS NULL`

	getErr := database.RMS.Get(&archivedAt, SQL, sessionID)
	return archivedAt, getErr
}

func DeleteUserSession(sessionID string) error {
	SQL := `UPDATE user_session
			  SET archived_at = NOW()
			  WHERE id = $1
			    AND archived_at IS NULL`

	_, delErr := database.RMS.Exec(SQL, sessionID)
	return delErr
}

func GetAllUsersByAdmin() ([]models.User, error) {
	SQL := `SELECT id, name, email, role 
			FROM users
    	      WHERE role = 'user' 
    	        AND archived_at IS NULL`

	users := make([]models.User, 0)
	if fetchErr := database.RMS.Select(&users, SQL); fetchErr != nil {
		return users, fetchErr
	}

	SQL = `SELECT id, address, latitude, longitude, user_id 
			FROM address
    	      WHERE archived_at IS NULL`

	addresses := make([]models.Address, 0)
	if fetchErr := database.RMS.Select(&addresses, SQL); fetchErr != nil {
		return users, fetchErr
	}

	addressMap := make(map[string][]models.Address)
	for _, addr := range addresses {
		addressMap[addr.UserID] = append(addressMap[addr.UserID], addr)
	}

	for i := range users {
		if userAddresses, exists := addressMap[users[i].ID]; exists {
			users[i].Address = userAddresses
		}
	}

	return users, nil
}

func GetAllUsersBySubAdmin(loggedUserID string) ([]models.User, error) {
	SQL := `SELECT id, name, email, role 
			FROM users
    	      WHERE created_by = $1
    	        AND archived_at IS NULL`

	users := make([]models.User, 0)
	if fetchErr := database.RMS.Select(&users, SQL, loggedUserID); fetchErr != nil {
		return users, fetchErr
	}

	SQL = `SELECT a.id, a.address, a.latitude, a.longitude, a.user_id
			FROM address a
					 JOIN users u on a.user_id = u.id
			WHERE created_by = $1
			  AND a.archived_at IS NULL
			  AND u.archived_at IS NULL`

	addresses := make([]models.Address, 0)
	if fetchErr := database.RMS.Select(&addresses, SQL, loggedUserID); fetchErr != nil {
		return users, fetchErr
	}

	addressMap := make(map[string][]models.Address)
	for _, addr := range addresses {
		addressMap[addr.UserID] = append(addressMap[addr.UserID], addr)
	}

	for i := range users {
		if userAddresses, exists := addressMap[users[i].ID]; exists {
			users[i].Address = userAddresses
		}
	}

	return users, nil
}

func GetUserCoordinates(userAddressID string) (models.Coordinates, error) {
	SQL := `SELECT latitude, longitude 
              FROM address 
              WHERE id = $1
              	AND archived_at IS NULL`

	var coordinates models.Coordinates
	getErr := database.RMS.Get(&coordinates, SQL, userAddressID)
	return coordinates, getErr
}

func GetRestaurantCoordinates(restaurantAddressID string) (models.Coordinates, error) {
	SQL := `SELECT latitude, longitude 
              FROM restaurants 
              WHERE id = $1
              	AND archived_at IS NULL`

	var coordinates models.Coordinates
	getErr := database.RMS.Get(&coordinates, SQL, restaurantAddressID)
	return coordinates, getErr
}

func CalculateDistance(userCoordinates, restaurantCoordinates models.Coordinates) (float64, error) {
	args := []interface{}{userCoordinates.Latitude, userCoordinates.Longitude,
		restaurantCoordinates.Latitude, restaurantCoordinates.Longitude}

	SQL := `SELECT ROUND(
						   (earth_distance(
									ll_to_earth($1, $2),
									ll_to_earth($3, $4)
							) / 1000.0)::numeric, 1
				   ) AS distance_km`

	var distance float64
	getErr := database.RMS.Get(&distance, SQL, args...)
	return distance, getErr
}
