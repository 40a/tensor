package rbac

import (
	"github.com/pearsonappeng/tensor/db"
	"github.com/pearsonappeng/tensor/models/common"

	log "github.com/Sirupsen/logrus"
	"gopkg.in/mgo.v2/bson"
	"github.com/pearsonappeng/tensor/models"
)

func inventoryRead(user common.User, inventory models.RootModel) bool {
	// allow access if the user is super user or
	// a system auditor
	if user.IsSuperUser || user.IsSystemAuditor {
		return true
	}

	// check whether the user is an member of the objects' organization
	// since this is read it doesn't matter what permission assigned to the user
	count, err := db.Organizations().Find(bson.M{
		"roles.user_id": user.ID,
		"organization_id": inventory.GetOrganizationID(),
		"roles.role": OrganizationAdmin,
	}).Count()
	if err != nil {
		log.Errorln("Error while checking the user and organizational memeber:", err)
	}
	if count > 0 {
		return true
	}

	var teams []bson.ObjectId
	// Check whether the user has access to object
	// using roles list
	// if object has granted team get those teams to list
	for _, v := range inventory.GetRoles() {
		if v.Type == "team" {
			teams = append(teams, v.GranteeID)
		}

		if v.Type == "user" && v.GranteeID == user.ID {
			return true
		}
	}

	//check team permissions if, the user is in a team assign indirect permissions
	count, err = db.Teams().Find(bson.M{
		"_id:": bson.M{"$in": teams},
		"organization_id": inventory.GetOrganizationID(),
		"roles.user_id": user.ID,
	}).Count()
	if err != nil {
		log.Errorln("Error while checking the user is granted teams' memeber:", err)
	}
	if count > 0 {
		return true
	}

	return false
}

func inventoryWrite(user common.User, inventory models.RootModel) bool {
	// Allow access if the user is super user or
	// a system auditor
	if user.IsSuperUser {
		return true
	}

	// check whether the user is an member of the objects' organization
	// since this is write permission it is must user need to be an admin
	count, err := db.Organizations().Find(bson.M{
		"roles.user_id": user.ID,
		"organization_id": inventory.GetOrganizationID(),
		"roles.role": OrganizationAdmin,
	}).Count()
	if err != nil {
		log.Errorln("Error while checking the user and organizational admin:", err)
	}
	if count > 0 {
		return true
	}

	var teams []bson.ObjectId
	// Check whether the user has access to object
	// using roles list
	// if object has granted team get those teams to list
	for _, v := range inventory.GetRoles() {
		if v.Type == "team" && (v.Role == InventoryAdmin || v.Role == InventoryUpdate) {
			teams = append(teams, v.GranteeID)
		}

		if v.Type == "user" && v.GranteeID == user.ID && (v.Role == InventoryAdmin || v.Role == InventoryUpdate) {
			return true
		}
	}

	// Check team permissions of the user,
	// and team has admin and update privileges
	query := bson.M{"_id:": bson.M{"$in": teams}, "roles.user_id": user.ID}
	count, err = db.Teams().Find(query).Count()
	if err != nil {
		log.Errorln("Error while checking the user is granted teams' memeber:", err)
	}
	if count > 0 {
		return true
	}

	return false
}

func inventoryUse(user common.User, inventory models.RootModel) bool {
	// allow access if the user is super user or
	// a system auditor
	if user.IsSuperUser {
		return true
	}

	// check whether the user is an member of the objects' organization
	// since this is write permission it is must user need to be an admin
	count, err := db.Organizations().Find(bson.M{
		"roles.user_id": user.ID,
		"organization_id": inventory.GetOrganizationID(),
		"roles.role": OrganizationAdmin,
	}).Count()
	if err != nil {
		log.Errorln("Error while checking the user and organizational admin:", err)
	}
	if count > 0 {
		return true
	}

	//teams which has relevant permissions
	var teams []bson.ObjectId
	// check whether the user has access to object
	// using roles list
	// if object has granted team get those teams to list
	for _, v := range inventory.GetRoles() {
		if v.Type == "team" && (v.Role == InventoryAdmin || v.Role == InventoryUse) {
			teams = append(teams, v.GranteeID)
		}

		if v.Type == "user" && v.GranteeID == user.ID && (v.Role == InventoryAdmin || v.Role == InventoryUse) {
			return true
		}
	}

	// check team permissions of the user,
	// and team has admin and update privileges
	query := bson.M{"_id:": bson.M{"$in": teams}, "roles.user_id": user.ID}
	count, err = db.Teams().Find(query).Count()

	if err != nil {
		log.Errorln("Error while checking the user is granted teams' memeber:", err)
	}

	if count > 0 {
		return true
	}

	return false
}