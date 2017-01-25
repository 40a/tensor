package jtemplate

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pearsonappeng/tensor/api/helpers"
	thelpers "github.com/pearsonappeng/tensor/api/helpers/terraform"
	metadata "github.com/pearsonappeng/tensor/api/metadata/terraform"
	"github.com/pearsonappeng/tensor/db"
	"github.com/pearsonappeng/tensor/jwt"
	"github.com/pearsonappeng/tensor/models/ansible"
	"github.com/pearsonappeng/tensor/models/common"
	"github.com/pearsonappeng/tensor/models/terraform"

	log "github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/pearsonappeng/tensor/queue"
	"github.com/pearsonappeng/tensor/runners"
	"github.com/pearsonappeng/tensor/util"
	"gopkg.in/mgo.v2/bson"
)

// Keys for credential releated items stored in the Gin Context
const (
	CTXJobTemplate   = "job_template"
	CTXUser          = "user"
	CTXJobTemplateID = "job_template_id"
)

// Middleware generates a middleware handler function that works inside of a Gin request.
// This function takes CTXJobTemplateID from Gin Context and retrieves terraform job template data from the collection
// and store terraform job template data under key CTXJobTemplate in Gin Context
func Middleware(c *gin.Context) {
	ID, err := util.GetIdParam(CTXJobTemplateID, c)

	if err != nil {
		log.WithFields(log.Fields{
			"Terraform Job Template ID": ID,
			"Error":                     err.Error(),
		}).Errorln("Error while getting Terraform Job Template ID url parameter")
		c.JSON(http.StatusNotFound, common.Error{
			Code:     http.StatusNotFound,
			Messages: []string{"Not Found"},
		})
		c.Abort()
		return
	}

	var jobTemplate terraform.JobTemplate
	if err = db.TerrafromJobTemplates().FindId(bson.ObjectIdHex(ID)).One(&jobTemplate); err != nil {
		log.WithFields(log.Fields{
			"Terraform Job Template ID": ID,
			"Error":                     err.Error(),
		}).Errorln("Error while retriving Terraform Job Template form the database")
		c.JSON(http.StatusNotFound, common.Error{
			Code:     http.StatusNotFound,
			Messages: []string{"Not Found"},
		})
		c.Abort()
		return
	}

	// Set the Job Template to the gin.Context
	c.Set(CTXJobTemplate, jobTemplate)
	c.Next() //move to next pending handler
}

// GetJTemplate is a Gin handler function which returns the Terraform Job Template as a JSON object
// A success will return 200 status code
// A failure will return 500 status code
func GetJTemplate(c *gin.Context) {
	jobTemplate := c.MustGet(CTXJobTemplate).(terraform.JobTemplate)

	metadata.JTemplateMetadata(&jobTemplate)

	c.JSON(http.StatusOK, jobTemplate)
}

// GetJTemplates is a Gin handler function which returns list of Job Templates
// The resulting data structure contains:
// {
//  "count\": 99,
//  "next\": null,
//  "previous\": null,
//  "results\": [
// 	...
// 	]
// The `count` field indicates the total number of job templates
// found for the given query.  The `next` and `previous` fields provides links to
// additional results if there are more than will fit on a single page.  The
// `results` list contains zero or more job template records.
// A success returns 200 status code
// A failure returns 500 status code
// This takes lookup parameters and order parameters to filter and sort output data
func GetJTemplates(c *gin.Context) {
	user := c.MustGet(CTXUser).(common.User)

	parser := util.NewQueryParser(c)
	match := bson.M{}
	match = parser.Lookups([]string{"name", "description", "labels"}, match)

	query := db.JobTemplates().Find(match) // prepare the query
	// set sort value to the query based on request parameters
	if order := parser.OrderBy(); order != "" {
		query.Sort(order)
	}

	log.WithFields(log.Fields{
		"Query": query,
	}).Debugln("Parsed query")

	var jobTemplates []terraform.JobTemplate
	// new mongodb iterator
	iter := query.Iter()
	// loop through each result and modify for our needs
	var tmpJobTemplate terraform.JobTemplate
	// iterate over all and only get valid objects
	for iter.Next(&tmpJobTemplate) {
		metadata.JTemplateMetadata(&tmpJobTemplate)
		// good to go add to list
		jobTemplates = append(jobTemplates, tmpJobTemplate)
	}
	if err := iter.Close(); err != nil {
		log.WithFields(log.Fields{
			"Error": err.Error(),
		}).Errorln("Error while retriving Terraform Job Template data from the database")
		c.JSON(http.StatusInternalServerError, common.Error{
			Code:     http.StatusInternalServerError,
			Messages: []string{"Error while getting Terraform Job Template"},
		})
		return
	}

	count := len(jobTemplates)
	pgi := util.NewPagination(c, count)
	//if page is incorrect return 404
	if pgi.HasPage() {
		log.WithFields(log.Fields{
			"Page number": pgi.Page(),
		}).Debugln("Terraform job template page does not exist")
		c.JSON(http.StatusNotFound, gin.H{"detail": "Invalid page " + strconv.Itoa(pgi.Page()) + ": That page contains no results."})
		return
	}

	log.WithFields(log.Fields{
		"Count":    count,
		"Next":     pgi.NextPage(),
		"Previous": pgi.PreviousPage(),
		"Skip":     pgi.Skip(),
		"Limit":    pgi.Limit(),
	}).Debugln("Response info")
	// send response with JSON rendered data
	c.JSON(http.StatusOK, common.Response{
		Count:    count,
		Next:     pgi.NextPage(),
		Previous: pgi.PreviousPage(),
		Results:  jobTemplates[pgi.Skip():pgi.End()],
	})
}

// AddJTemplate is Gin handler function which creates a new Credential using request payload
// This accepts Job Template model.
// fields to create a new job template:
// name: Name of this job template. string, required
// description: Optional description of this job template. string, default=""
// job_type:  choice
//   - `run: Run default
//   - `check: Check
//   - `scan: Scan
// inventory:  bson.ObjectId, default=nil
// project:  bson.ObjectId, default=nil
// playbook:  string, default=""
// credential:  bson.ObjectId, default=nil
// cloud_credential:  bson.ObjectId, default=nil
// network_credential:  bson.ObjectId, default=nil
// forks:  integer, default=0
// limit:  string, default=""
// verbosity:  choice
//   - 0: 0 Normal default
//   - 1: 1 Verbose
//   - 2: 2 More Verbose
//   - 3: 3 Debug
//   - 4: 4 Connection Debug
//   - 5: 5 WinRM Debug
// extra_vars:  string, default=""
// job_tags:  string, default=""
// force_handlers:  boolean, default=False
// skip_tags:  string, default=""
// start_at_task:  string, default=""
// host_config_key:  string, default=""
// ask_variables_on_launch:  boolean, default=False
// ask_limit_on_launch:  boolean, default=False
// ask_tags_on_launch:  boolean, default=False
// ask_skip_tags_on_launch:  boolean, default=False
// ask_job_type_on_launch:  boolean, default=False
// ask_inventory_on_launch:  boolean, default=False
// ask_credential_on_launch:  boolean, default=False
// become_enabled:  boolean, default=False
// allow_simultaneous:  boolean, default=False
func AddJTemplate(c *gin.Context) {
	var req terraform.JobTemplate
	// get user from the gin.Context
	user := c.MustGet(CTXUser).(common.User)

	err := binding.JSON.Bind(c.Request, &req)
	if err != nil {
		log.WithFields(log.Fields{
			"Error": err.Error(),
		}).Errorln("Invlid JSON request")
		// Return 400 if request has bad JSON format
		c.JSON(http.StatusBadRequest, common.Error{
			Code:     http.StatusBadRequest,
			Messages: util.GetValidationErrors(err),
		})
		return
	}

	// check the project exist or not
	if !helpers.ProjectExist(req.ProjectID) {
		c.JSON(http.StatusInternalServerError, common.Error{
			Code:     http.StatusInternalServerError,
			Messages: []string{"Project does not exists"},
		})
		return
	}

	// if the JobTemplate exist in the collection it is not unique
	if thelpers.IsNotUniqueJTemplate(req.Name, req.ProjectID) {
		c.JSON(http.StatusBadRequest, common.Error{
			Code:     http.StatusBadRequest,
			Messages: []string{"Terraform Job Template with this Name already exists."},
		})
		return
	}

	// check the machine credential exist or not
	if !helpers.MachineCredentialExist(req.MachineCredentialID) {
		c.JSON(http.StatusInternalServerError, common.Error{
			Code:     http.StatusInternalServerError,
			Messages: []string{"Machine Credential does not exists"},
		})
		return
	}

	// check the network credential exist or not
	if req.NetworkCredentialID != nil {
		if !helpers.NetworkCredentialExist(*req.NetworkCredentialID) {
			c.JSON(http.StatusInternalServerError, common.Error{
				Code:     http.StatusInternalServerError,
				Messages: []string{"Network Credential does not exists"},
			})
			return
		}
	}

	// check the network credential exist or not
	if req.CloudCredentialID != nil {
		if !helpers.CloudCredentialExist(*req.CloudCredentialID) {
			c.JSON(http.StatusInternalServerError, common.Error{
				Code:     http.StatusInternalServerError,
				Messages: []string{"Cloud Credential does not exists"},
			})
			return
		}
	}

	req.ID = bson.NewObjectId()
	req.Created = time.Now()
	req.Modified = time.Now()
	req.CreatedByID = user.ID
	req.ModifiedByID = user.ID

	// insert new object
	if err = db.TerrafromJobTemplates().Insert(req); err != nil {
		log.WithFields(log.Fields{
			"Terraform Job Template ID": req.ID.Hex(),
			"Error":                     err.Error(),
		}).Errorln("Error while creating Terraform Job Template")
		c.JSON(http.StatusInternalServerError, common.Error{
			Code:     http.StatusInternalServerError,
			Messages: []string{"Error while creating Terraform Job Template"},
		})
		return
	}

	// add new activity to activity stream
	if err := db.ActivityStream().Insert(common.Activity{
		ID:          bson.NewObjectId(),
		ActorID:     user.ID,
		Type:        CTXJobTemplate,
		ObjectID:    req.ID,
		Description: "Terraform Job Template " + req.Name + " created",
		Created:     time.Now(),
	}); err != nil {
		log.WithFields(log.Fields{
			"Error": err.Error(),
		}).Errorln("Failed to add new Activity")
	}
	// set `related` and `summary` feilds
	metadata.JTemplateMetadata(&req)

	// send response with JSON rendered data
	c.JSON(http.StatusCreated, req)
}

// UpdateJTemplate is a Gin handler function which updates a Job Template using request payload
// A success returns 200 status code
// A failure returns 500 status code
// if the request body is invalid returns serialized Error model with 400 status code
func UpdateJTemplate(c *gin.Context) {
	// get template from the gin.Context
	jobTemplate := c.MustGet(CTXJobTemplate).(terraform.JobTemplate)
	// get user from the gin.Context
	user := c.MustGet(CTXUser).(common.User)

	var req terraform.JobTemplate
	if err := binding.JSON.Bind(c.Request, &req); err != nil {
		c.JSON(http.StatusBadRequest, common.Error{
			Code:     http.StatusBadRequest,
			Messages: util.GetValidationErrors(err),
		})
		return
	}

	// check the project exist or not
	if !helpers.ProjectExist(req.ProjectID) {
		c.JSON(http.StatusInternalServerError, common.Error{
			Code:     http.StatusInternalServerError,
			Messages: []string{"Project does not exists"},
		})
		return
	}

	if req.Name != jobTemplate.Name {
		// if the JobTemplate exist in the collection it is not unique
		if thelpers.IsNotUniqueJTemplate(req.Name, req.ProjectID) {
			c.JSON(http.StatusBadRequest, common.Error{
				Code:     http.StatusBadRequest,
				Messages: []string{"Terraform Job Template with this Name already exists."},
			})
			return
		}
	}

	// check whether the machine credential exist or not
	if !helpers.MachineCredentialExist(req.MachineCredentialID) {
		c.JSON(http.StatusInternalServerError, common.Error{
			Code:     http.StatusInternalServerError,
			Messages: []string{"Machine Credential does not exists"},
		})
		return
	}

	// check whether the network credential exist or not
	if req.NetworkCredentialID != nil {
		if !helpers.NetworkCredentialExist(*req.NetworkCredentialID) {
			c.JSON(http.StatusInternalServerError, common.Error{
				Code:     http.StatusInternalServerError,
				Messages: []string{"Network Credential does not exists"},
			})
			return
		}
	}

	// check whether the network credential exist or not
	if req.CloudCredentialID != nil {
		if !helpers.CloudCredentialExist(*req.CloudCredentialID) {
			c.JSON(http.StatusInternalServerError, common.Error{
				Code:     http.StatusInternalServerError,
				Messages: []string{"Cloud Credential does not exists"},
			})
			return
		}
	}

	jobTemplate.Name = strings.Trim(req.Name, " ")
	jobTemplate.JobType = req.JobType
	jobTemplate.ProjectID = req.ProjectID
	jobTemplate.MachineCredentialID = req.MachineCredentialID
	jobTemplate.Description = strings.Trim(req.Description, " ")
	jobTemplate.Vars = req.Vars
	jobTemplate.PromptVariables = req.PromptVariables
	jobTemplate.CloudCredentialID = req.CloudCredentialID
	jobTemplate.NetworkCredentialID = req.NetworkCredentialID
	jobTemplate.PromptCredential = req.PromptCredential
	jobTemplate.PromptJobType = req.PromptJobType
	jobTemplate.AllowSimultaneous = req.AllowSimultaneous

	jobTemplate.Modified = time.Now()
	jobTemplate.ModifiedByID = user.ID

	// update object
	if err := db.TerrafromJobTemplates().UpdateId(jobTemplate.ID, jobTemplate); err != nil {
		log.WithFields(log.Fields{
			"Terraform Job Template ID": req.ID.Hex(),
			"Error":                     err.Error(),
		}).Errorln("Error while updating Terraform Job Template")
		c.JSON(http.StatusInternalServerError, common.Error{
			Code:     http.StatusInternalServerError,
			Messages: []string{"Error while updating Terraform Job Template"},
		})
		return
	}

	// add new activity to activity stream
	if err := db.ActivityStream().Insert(common.Activity{
		ID:          bson.NewObjectId(),
		ActorID:     user.ID,
		Type:        CTXJobTemplate,
		ObjectID:    req.ID,
		Description: "Terraform Job Template " + jobTemplate.Name + " updated",
		Created:     time.Now(),
	}); err != nil {
		log.WithFields(log.Fields{
			"Error": err.Error(),
		}).Errorln("Failed to add new Activity")
	}

	// set `related` and `summary` feilds
	metadata.JTemplateMetadata(&jobTemplate)

	// send response with JSON rendered data
	c.JSON(http.StatusOK, jobTemplate)
}

// PatchJTemplate is a Gin handler function which partially updates a Job Template using request payload.
// patch will only update feilds which included in the POST body
// A success returns 200 status code
// A failure returns 500 status code
// if the request body is invalid returns serialized Error model with 400 status code
func PatchJTemplate(c *gin.Context) {
	// get template from the gin.Context
	jobTemplate := c.MustGet(CTXJobTemplate).(terraform.JobTemplate)
	// get user from the gin.Context
	user := c.MustGet(CTXUser).(common.User)

	var req terraform.PatchJobTemplate
	if err := binding.JSON.Bind(c.Request, &req); err != nil {
		c.JSON(http.StatusBadRequest, common.Error{
			Code:     http.StatusBadRequest,
			Messages: util.GetValidationErrors(err),
		})
		return
	}

	// check the project exist or not
	if req.ProjectID != nil && !helpers.ProjectExist(*req.ProjectID) {
		c.JSON(http.StatusInternalServerError, common.Error{
			Code:     http.StatusInternalServerError,
			Messages: []string{"Project does not exists"},
		})
		return
	}

	if req.Name != nil && *req.Name != jobTemplate.Name {
		// if the JobTemplate exist in the collection it is not unique
		projectID := jobTemplate.ProjectID
		if req.ProjectID != nil {
			projectID = *req.ProjectID
		}

		if helpers.IsNotUniqueJTemplate(*req.Name, projectID) {
			c.JSON(http.StatusBadRequest, common.Error{
				Code:     http.StatusBadRequest,
				Messages: []string{"Terraform Job Template with this Name already exists."},
			})
			return
		}
	}

	if req.MachineCredentialID != nil {
		// check whether the machine credential exist or not
		if !helpers.MachineCredentialExist(*req.MachineCredentialID) {
			c.JSON(http.StatusBadRequest, common.Error{
				Code:     http.StatusBadRequest,
				Messages: []string{"Machine Credential does not exists."},
			})
			return
		}
	}

	// check whether the network credential exist or not
	if req.NetworkCredentialID != nil {
		if !helpers.NetworkCredentialExist(*req.NetworkCredentialID) {
			c.JSON(http.StatusBadRequest, common.Error{
				Code:     http.StatusBadRequest,
				Messages: []string{"Network Credential does not exists."},
			})
			return
		}
	}

	// check whether the network credential exist or not
	if req.CloudCredentialID != nil {
		if !helpers.CloudCredentialExist(*req.CloudCredentialID) {
			c.JSON(http.StatusBadRequest, common.Error{
				Code:     http.StatusBadRequest,
				Messages: []string{"Cloud Credential does not exists."},
			})
			return
		}
	}

	if req.Name != nil {
		jobTemplate.Name = strings.Trim(*req.Name, " ")
	}

	if req.JobType != nil {
		jobTemplate.JobType = *req.JobType
	}

	if req.ProjectID != nil {
		jobTemplate.ProjectID = *req.ProjectID
	}

	if req.MachineCredentialID != nil {
		jobTemplate.MachineCredentialID = *req.MachineCredentialID
	}

	if req.Description != nil {
		jobTemplate.Description = strings.Trim(*req.Description, " ")
	}

	if req.Vars != nil {
		jobTemplate.Vars = *req.Vars
	}

	if req.PromptVariables != nil {
		jobTemplate.PromptVariables = *req.PromptVariables
	}

	if req.CloudCredentialID != nil {
		// if empty string then make the credential null
		if len(*req.CloudCredentialID) == 12 {
			jobTemplate.CloudCredentialID = req.CloudCredentialID
		} else {
			jobTemplate.CloudCredentialID = nil
		}
	}

	if req.NetworkCredentialID != nil {
		// if empty string then make the credential null
		if len(*req.NetworkCredentialID) == 12 {
			jobTemplate.NetworkCredentialID = req.NetworkCredentialID
		} else {
			jobTemplate.NetworkCredentialID = nil
		}
	}

	if req.PromptCredential != nil {
		jobTemplate.PromptCredential = *req.PromptCredential
	}

	if req.PromptJobType != nil {
		jobTemplate.PromptJobType = *req.PromptJobType
	}

	if req.AllowSimultaneous != nil {
		jobTemplate.AllowSimultaneous = *req.AllowSimultaneous
	}

	jobTemplate.Modified = time.Now()
	jobTemplate.ModifiedByID = user.ID

	// update object
	if err := db.TerrafromJobTemplates().UpdateId(jobTemplate.ID, jobTemplate); err != nil {
		log.WithFields(log.Fields{
			"Terraform Job Template ID": jobTemplate.ID.Hex(),
			"Error":                     err.Error(),
		}).Errorln("Error while updating Terraform Job Template")
		c.JSON(http.StatusInternalServerError, common.Error{
			Code:     http.StatusInternalServerError,
			Messages: []string{"Error while updating Terraform Job Template"},
		})
		return
	}

	// add new activity to activity stream
	if err := db.ActivityStream().Insert(common.Activity{
		ID:          bson.NewObjectId(),
		ActorID:     user.ID,
		Type:        CTXJobTemplate,
		ObjectID:    jobTemplate.ID,
		Description: "Terraform Job Template " + jobTemplate.Name + " updated",
		Created:     time.Now(),
	}); err != nil {
		log.WithFields(log.Fields{
			"Error": err.Error(),
		}).Errorln("Failed to add new Activity")
	}

	// set `related` and `summary` feilds
	metadata.JTemplateMetadata(&jobTemplate)

	// send response with JSON rendered data
	c.JSON(http.StatusOK, jobTemplate)
}

// RemoveJTemplate is a Gin handler function which removes a Job Template object from the database
// A success returns 204 status code
// A failure returns 500 status code
func RemoveJTemplate(c *gin.Context) {
	// get template from the gin.Context
	jobTemplate := c.MustGet(CTXJobTemplate).(terraform.JobTemplate)
	// get user from the gin.Context
	user := c.MustGet(CTXUser).(common.User)

	// remove object from the collection
	if err := db.TerrafromJobTemplates().RemoveId(jobTemplate.ID); err != nil {
		log.WithFields(log.Fields{
			"Terraform Job Template ID": jobTemplate.ID.Hex(),
			"Error":                     err.Error(),
		}).Errorln("Error while removing Terraform Job Temlate")
		c.JSON(http.StatusInternalServerError, common.Error{
			Code:     http.StatusInternalServerError,
			Messages: []string{"Error while removing Terraform Job Template"},
		})
		return
	}

	// add new activity to activity stream
	if err := db.ActivityStream().Insert(common.Activity{
		ID:          bson.NewObjectId(),
		ActorID:     user.ID,
		Type:        CTXJobTemplate,
		ObjectID:    user.ID,
		Description: "Terraform Job Template " + jobTemplate.Name + " deleted",
		Created:     time.Now(),
	}); err != nil {
		log.WithFields(log.Fields{
			"Error": err.Error(),
		}).Errorln("Failed to add new Activity")
	}

	// abort with 204 status code
	c.AbortWithStatus(http.StatusNoContent)
}

// ActivityStream returns serialized list of Activity models associated with the Job Template
// Resulting data structure contains:
// {
//  "count\": 99,
//  "next\": null,
//  "previous\": null,
//  "results\": [
// 	...
// 	]
// }
// The `count` field indicates the total number of activity streams found for the given query.
// The `next` and `previous` fields provides links to additional results if there are more than will fit on a single page.
// The `results` list contains zero or more activity stream records.
// success returns 200 status code
// failure reruns 500 status code
//
func ActivityStream(c *gin.Context) {
	jobTemplate := c.MustGet(CTXJobTemplate).(terraform.JobTemplate)

	var activities []common.Activity
	err := db.ActivityStream().Find(bson.M{"object_id": jobTemplate.ID, "type": CTXJobTemplate}).All(&activities)

	if err != nil {
		log.WithFields(log.Fields{
			"Error": err.Error(),
		}).Errorln("Error while retriving Activity data from the database")
		c.JSON(http.StatusInternalServerError, common.Error{
			Code:     http.StatusInternalServerError,
			Messages: []string{"Error while Activities"},
		})
		return
	}

	count := len(activities)
	pgi := util.NewPagination(c, count)
	//if page is incorrect return 404
	if pgi.HasPage() {
		log.WithFields(log.Fields{
			"Page number": pgi.Page(),
		}).Debugln("Activity Stream page does not exist")
		c.JSON(http.StatusNotFound, gin.H{"detail": "Invalid page " + strconv.Itoa(pgi.Page()) + ": That page contains no results."})
		return
	}
	// send response with JSON rendered data
	c.JSON(http.StatusOK, common.Response{
		Count:    count,
		Next:     pgi.NextPage(),
		Previous: pgi.PreviousPage(),
		Results:  activities[pgi.Skip():pgi.End()],
	})
}

// Jobs returns list of jobs associated with the Job Template
// Resulting data structure contains:
// {
//  "count\": 99,
//  "next\": null,
//  "previous\": null,
//  "results\": [
// 	...
// 	]
// }
// The `count` field indicates the total number of jobs found for the given query.
// The `next` and `previous` fields provides links to additional results if there are more than will fit on a single page.
// The `results` list contains zero or more job records.
func Jobs(c *gin.Context) {
	jobTemplate := c.MustGet(CTXJobTemplate).(terraform.JobTemplate)

	var jbs []terraform.Job
	// new mongodb iterator
	iter := db.TerrafromJobs().Find(bson.M{"job_template_id": jobTemplate.ID}).Iter()
	// loop through each result and modify for our needs
	var tmpJob terraform.Job
	// iterate over all and only get valid objects
	for iter.Next(&tmpJob) {
		metadata.JobMetadata(&tmpJob)
		// good to go add to list
		jbs = append(jbs, tmpJob)
	}

	if err := iter.Close(); err != nil {
		log.WithFields(log.Fields{
			"Error": err.Error(),
		}).Errorln("Error while retriving Terraform jobs data from the database")
		c.JSON(http.StatusInternalServerError, common.Error{
			Code:     http.StatusInternalServerError,
			Messages: []string{"Error while getting Terraform Jobs"},
		})
		return
	}

	count := len(jbs)
	pgi := util.NewPagination(c, count)
	//if page is incorrect return 404
	if pgi.HasPage() {
		log.WithFields(log.Fields{
			"Page number": pgi.Page(),
		}).Debugln("Job page does not exist")
		c.JSON(http.StatusNotFound, gin.H{"detail": "Invalid page " + strconv.Itoa(pgi.Page()) + ": That page contains no results."})
		return
	}
	// send response with JSON rendered data
	c.JSON(http.StatusOK, common.Response{
		Count:    count,
		Next:     pgi.NextPage(),
		Previous: pgi.PreviousPage(),
		Results:  jbs[pgi.Skip():pgi.End()],
	})
}

// Launch creates a new job and adds the job into job queue. If any
// passwords, inventory, or extra variables (extra_vars) are required, they must
// be passed via POST data, with extra_vars given as a JSON string.
// If `credential_needed_to_start` is `true` then the `credential` field is required
// and if the `inventory_needed_to_start` is `True` then the `inventory` is required as well.
// success returns JSON serialized Job model with 201 status code
// if the request body is invalid returns JSON serialized Error model with 400 status code
func Launch(c *gin.Context) {
	// get job template that was set by the Middleware
	template := c.MustGet(CTXJobTemplate).(terraform.JobTemplate)
	// get user object set by the jwt Middleware
	user := c.MustGet(CTXUser).(common.User)

	// create a new Launch model
	var req terraform.Launch
	// if the body present deserialize it
	if err := binding.JSON.Bind(c.Request, &req); err != nil {
		// accept nil request body for POST request, since all the feilds are optional
		if err != io.EOF {
			// Return 400 if request has bad JSON
			// and return formatted validation errors
			c.JSON(http.StatusBadRequest, common.Error{
				Code:     http.StatusBadRequest,
				Messages: util.GetValidationErrors(err),
			})
			return // abort
		}
	}

	// create new Job
	job := terraform.Job{
		ID:                  bson.NewObjectId(),
		Name:                template.Name,
		Description:         template.Description,
		LaunchType:          "manual",
		CancelFlag:          false,
		Status:              "new",
		JobType:             ansible.JOBTYPE_ANSIBLE_JOB,
		Vars:                template.Vars,
		MachineCredentialID: template.MachineCredentialID,
		JobTemplateID:       template.ID,
		ProjectID:           template.ProjectID,
		NetworkCredentialID: template.NetworkCredentialID,
		CloudCredentialID:   template.CloudCredentialID,
		SCMCredentialID:     nil,
		CreatedByID:         user.ID,
		ModifiedByID:        user.ID,
		Created:             time.Now(),
		Modified:            time.Now(),
		PromptCredential:    template.PromptCredential,
		PromptJobType:       template.PromptJobType,
		PromptVariables:     template.PromptVariables,
		AllowSimultaneous:   template.AllowSimultaneous,
	}

	// if prompt is true override Job template
	// if not provided return an error message
	if template.PromptVariables {
		if !(len(req.Vars) > 0) {
			c.JSON(http.StatusBadRequest, common.Error{
				Code:     http.StatusBadRequest,
				Messages: []string{"Additional variables required"},
			})
			return
		}

		job.Vars = req.Vars
	}

	if template.PromptJobType {
		if !(len(req.JobType) > 0) {
			c.JSON(http.StatusBadRequest, common.Error{
				Code:     http.StatusBadRequest,
				Messages: []string{"Job Type required"},
			})
			return
		}

		job.JobType = req.JobType
	}

	// create new Ansible runner Job
	runnerJob := runners.QueueJob{
		Job:      job,
		Template: template,
		User:     user,
	}

	if template.PromptCredential {
		if len(req.MachineCredentialID) != 24 {
			c.JSON(http.StatusBadRequest, common.Error{
				Code:     http.StatusBadRequest,
				Messages: []string{"Credential required"},
			})
			return
		}
		job.MachineCredentialID = req.MachineCredentialID
	}

	if job.NetworkCredentialID != nil {
		var credential common.Credential
		err := db.Credentials().FindId(*job.NetworkCredentialID).One(&credential)
		if err != nil {
			log.WithFields(log.Fields{
				"Error": err.Error(),
			}).Errorln("Error while getting Network Credential")
			c.JSON(http.StatusInternalServerError, common.Error{
				Code:     http.StatusInternalServerError,
				Messages: []string{"Error while getting Network Credential"},
			})
			return
		}
		runnerJob.NetworkCred = credential
	}

	if job.CloudCredentialID != nil {
		var credential common.Credential
		err := db.Credentials().FindId(*job.CloudCredentialID).One(&credential)
		if err != nil {
			log.WithFields(log.Fields{
				"Error": err.Error(),
			}).Errorln("Error while getting Cloud Credential")
			c.JSON(http.StatusInternalServerError, common.Error{
				Code:     http.StatusInternalServerError,
				Messages: []string{"Error while getting Cloud Credential"},
			})
			return
		}
		runnerJob.CloudCred = credential
	}

	var credential common.Credential
	if err := db.Credentials().FindId(job.MachineCredentialID).One(&credential); err != nil {
		log.WithFields(log.Fields{
			"Error": err.Error(),
		}).Errorln("Error while getting Machine Credential")
		c.JSON(http.StatusInternalServerError, common.Error{
			Code:     http.StatusInternalServerError,
			Messages: []string{"Error while getting Machine Credential"},
		})
		return
	}
	runnerJob.MachineCred = credential

	// get project information
	var project common.Project
	if err := db.Projects().FindId(job.ProjectID).One(&project); err != nil {
		log.WithFields(log.Fields{
			"Error": err.Error(),
		}).Errorln("Error while getting Project")
		c.JSON(http.StatusInternalServerError, common.Error{
			Code:     http.StatusInternalServerError,
			Messages: []string{"Error while getting Project"},
		})
		return
	}
	runnerJob.Project = project

	// Get jwt token for authorize Ansible inventory plugin
	var token jwt.LocalToken
	if err := jwt.NewAuthToken(&token); err != nil {
		log.WithFields(log.Fields{
			"Error": err.Error(),
		}).Errorln("Error while getting Token")
		c.JSON(http.StatusInternalServerError, common.Error{
			Code:     http.StatusInternalServerError,
			Messages: []string{"Error while getting Token"},
		})
		return
	}
	runnerJob.Token = token.Token

	// Insert new job into jobs collection
	if err := db.TerrafromJobs().Insert(job); err != nil {
		log.WithFields(log.Fields{
			"Job ID": job.ID,
			"Error":  err.Error(),
		}).Errorln("Error while creating Terraform Job")
		c.JSON(http.StatusInternalServerError, common.Error{
			Code:     http.StatusInternalServerError,
			Messages: []string{"Error while creating Terraform Job"},
		})
		return
	}

	// update if requested
	if runnerJob.Project.ScmUpdateOnLaunch {
		tj, err := runners.UpdateProject(project)
		runnerJob.PreviousJob = tj
		if err != nil {
			log.WithFields(log.Fields{
				"Error": err.Error(),
			}).Errorln("Error while adding the job to job queue")
			c.JSON(http.StatusInternalServerError, common.Error{
				Code:     http.StatusInternalServerError,
				Messages: []string{"Error while creating Terraform Job"},
			})
			return
		}
	}

	// Add the job to queue
	jobQueue := queue.OpenJobQueue()
	jobBytes, err := json.Marshal(runnerJob)
	if err != nil {
		log.WithFields(log.Fields{
			"Error":    err.Error(),
			"Job Info": jobBytes,
		}).Errorln("Error while adding the job to job queue")
		c.JSON(http.StatusInternalServerError, common.Error{
			Code:     http.StatusInternalServerError,
			Messages: []string{"Error while creating Terraform Job"},
		})
		return
	}
	jobQueue.PublishBytes(jobBytes)

	// set additianl information to Job
	metadata.JobMetadata(&job)

	// return serialized job
	c.JSON(http.StatusCreated, job)
}

// LaunchInfo returns JSON serialized launch information to determine if the job_template can be
// launched and whether any passwords are required to launch the job_template.
//
// ask_variables_on_launch: Flag indicating whether the job template is configured to prompt for variables upon launch
// ask_tags_on_launch: Flag indicating whether the job template is configured to prompt for tags upon launch
// ask_skip_tags_on_launch: Flag indicating whether the job template is configured to prompt for skip_tags upon launch
// ask_job_type_on_launch: Flag indicating whether the job template is configured to prompt for job_type upon launch
// ask_limit_on_launch: Flag indicating whether the job template is configured to prompt for limit upon launch
// ask_inventory_on_launch: Flag indicating whether the job template is configured to prompt for inventory upon launch
// ask_credential_on_launch: Flag indicating whether the job template is configured to prompt for credential upon launch
// can_start_without_user_input: Flag indicating if the job template can be launched without user-input
// variables_needed_to_start: Required variable names required to launch the job_template
// credential_needed_to_start: Flag indicating the presence of a credential associated with the job template.
// If not then one should be supplied when launching the job
// inventory_needed_to_start: Flag indicating the presence of an inventory associated with the job template.
// If not then one should be supplied when launching the job
func LaunchInfo(c *gin.Context) {
	// get template from the gin.Context
	jt := c.MustGet(CTXJobTemplate).(terraform.JobTemplate)

	var isCredentialNeeded bool
	var isInventoryNeeded bool

	defaults := gin.H{
		"vars":     jt.Vars,
		"job_type": jt.JobType,
	}

	var cred common.Credential

	if err := db.Credentials().FindId(jt.MachineCredentialID).One(&cred); err != nil {
		log.WithFields(log.Fields{
			"Error": err.Error(),
		}).Errorln("Cound not find Credential")
		defaults["credential"] = nil
		isCredentialNeeded = true
	} else {
		defaults["credential"] = gin.H{
			"id":   cred.ID,
			"name": cred.Name,
		}
	}

	resp := gin.H{
		"passwords_needed_to_start":  []gin.H{},
		"ask_variables_on_launch":    jt.PromptVariables,
		"ask_job_type_on_launch":     jt.PromptJobType,
		"ask_credential_on_launch":   jt.PromptCredential,
		"variables_needed_to_start":  []gin.H{},
		"credential_needed_to_start": isCredentialNeeded,
		"job_template_data": gin.H{
			"id":          jt.ID.Hex(),
			"name":        jt.Name,
			"description": jt.Description,
		},
		"defaults": defaults,
	}

	// render JSON with 200 status code
	c.JSON(http.StatusOK, resp)
}

// ObjectRoles is a Gin handler function
// This returns available roles can be associated with a Job Template model
func ObjectRoles(c *gin.Context) {
	jobTemplate := c.MustGet(CTXJobTemplate).(terraform.JobTemplate)

	roles := []gin.H{
		{
			"type": "role",
			"related": gin.H{
				"job_template": "/v1/job_templates/" + jobTemplate.ID.Hex() + "/",
			},
			"summary_fields": gin.H{
				"resource_name":              jobTemplate.Name,
				"resource_type":              "job template",
				"resource_type_display_name": "Job Template",
			},
			"name":        "admin",
			"description": "Can manage all aspects of the job template",
		},
		{
			"type": "role",
			"related": gin.H{
				"job_template": "/v1/job_templates/" + jobTemplate.ID.Hex() + "/",
			},
			"summary_fields": gin.H{
				"resource_name":              jobTemplate.Name,
				"resource_type":              "job template",
				"resource_type_display_name": "Job Template",
			},
			"name":        "read",
			"description": "May view settings for the job template",
		},
		{
			"type": "role",
			"related": gin.H{
				"users":        "/api/v1/roles/22/users/",
				"job_template": "/v1/job_templates/" + jobTemplate.ID.Hex() + "/",
			},
			"summary_fields": gin.H{
				"resource_name":              jobTemplate.Name,
				"resource_type":              "job template",
				"resource_type_display_name": "Job Template",
			},
			"name":        "execute",
			"description": "May run the job template",
		},
	}

	count := len(roles)
	pgi := util.NewPagination(c, count)
	//if page is incorrect return 404
	if pgi.HasPage() {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Invalid page " + strconv.Itoa(pgi.Page()) + ": That page contains no results."})
		return
	}
	// send response with JSON rendered data
	c.JSON(http.StatusOK, common.Response{
		Count:    count,
		Next:     pgi.NextPage(),
		Previous: pgi.PreviousPage(),
		Results:  roles[pgi.Skip():pgi.End()],
	})

}
