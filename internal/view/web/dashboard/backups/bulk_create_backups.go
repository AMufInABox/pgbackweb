package backups

import (
	"net/http"
	"strconv"

	"github.com/eduardolat/pgbackweb/internal/service/backups"
	"github.com/eduardolat/pgbackweb/internal/staticdata"
	"github.com/eduardolat/pgbackweb/internal/validate"
	"github.com/eduardolat/pgbackweb/internal/view/web/component"
	"github.com/eduardolat/pgbackweb/internal/view/web/respondhtmx"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	nodx "github.com/nodxdev/nodxgo"
	alpine "github.com/nodxdev/nodxgo-alpine"
	htmx "github.com/nodxdev/nodxgo-htmx"
	lucide "github.com/nodxdev/nodxgo-lucide"
)

type bulkCreateBackupDTO struct {
	DatabaseIDs    []string `form:"database_ids"`
	DestinationID  string   `form:"destination_id"`
	IsLocal        string   `form:"is_local" validate:"required,oneof=true false"`
	NameTemplate   string   `form:"name_template" validate:"required"`
	CronExpression string   `form:"cron_expression" validate:"required"`
	TimeZone       string   `form:"time_zone" validate:"required"`
	IsActive       string   `form:"is_active" validate:"required,oneof=true false"`
	DestDir        string   `form:"dest_dir" validate:"required"`
	RetentionDays  string   `form:"retention_days"`
	OptDataOnly    string   `form:"opt_data_only" validate:"required,oneof=true false"`
	OptSchemaOnly  string   `form:"opt_schema_only" validate:"required,oneof=true false"`
	OptClean       string   `form:"opt_clean" validate:"required,oneof=true false"`
	OptIfExists    string   `form:"opt_if_exists" validate:"required,oneof=true false"`
	OptCreate      string   `form:"opt_create" validate:"required,oneof=true false"`
	OptNoComments  string   `form:"opt_no_comments" validate:"required,oneof=true false"`
}

func (h *handlers) bulkCreateBackupsHandler(c echo.Context) error {
	ctx := c.Request().Context()

	var formData bulkCreateBackupDTO
	if err := c.Bind(&formData); err != nil {
		return respondhtmx.ToastError(c, err.Error())
	}
	if err := validate.Struct(&formData); err != nil {
		return respondhtmx.ToastError(c, err.Error())
	}

	if len(formData.DatabaseIDs) == 0 {
		return respondhtmx.ToastError(c, "Please select at least one database")
	}

	// Parse retention days
	retentionDays, err := strconv.Atoi(formData.RetentionDays)
	if err != nil {
		return respondhtmx.ToastError(c, "Invalid retention days")
	}

	// Parse destination ID
	var destinationID *uuid.UUID
	if formData.DestinationID != "" {
		parsed, err := uuid.Parse(formData.DestinationID)
		if err != nil {
			return respondhtmx.ToastError(c, "Invalid destination ID")
		}
		destinationID = &parsed
	}

	// Create bulk backup request
	req := backups.BulkCreateBackupRequest{
		DatabaseIDs:    formData.DatabaseIDs,
		DestinationID:  destinationID,
		IsLocal:        formData.IsLocal == "true",
		NameTemplate:   formData.NameTemplate,
		CronExpression: formData.CronExpression,
		TimeZone:       formData.TimeZone,
		IsActive:       formData.IsActive == "true",
		DestDir:        formData.DestDir,
		RetentionDays:  int16(retentionDays),
		OptDataOnly:    formData.OptDataOnly == "true",
		OptSchemaOnly:  formData.OptSchemaOnly == "true",
		OptClean:       formData.OptClean == "true",
		OptIfExists:    formData.OptIfExists == "true",
		OptCreate:      formData.OptCreate == "true",
		OptNoComments:  formData.OptNoComments == "true",
	}

	// Create backups
	response, err := h.servs.BackupsService.BulkCreateBackups(ctx, req)
	if err != nil {
		return respondhtmx.ToastError(c, err.Error())
	}

	// Handle partial failures
	if len(response.Errors) > 0 {
		errorMsg := "Some backups failed to create: " + response.Errors[0]
		if len(response.Errors) > 1 {
			errorMsg += " (and " + strconv.Itoa(len(response.Errors)-1) + " more)"
		}
		return respondhtmx.ToastError(c, errorMsg)
	}

	successMsg := "Successfully created " + strconv.Itoa(len(response.CreatedBackups)) + " backup tasks"
	return respondhtmx.ToastSuccess(c, successMsg)
}

func (h *handlers) bulkCreateBackupDatabaseListHandler(c echo.Context) error {
	ctx := c.Request().Context()

	databases, err := h.servs.DatabasesService.GetAllDatabases(ctx)
	if err != nil {
		return c.HTML(http.StatusInternalServerError, `<div class="alert alert-error">Error loading databases</div>`)
	}

	if len(databases) == 0 {
		return c.HTML(http.StatusOK, `<div class="text-center py-4">No databases found</div>`)
	}

	html := `<div class="space-y-2 max-h-64 overflow-y-auto">`
	for _, db := range databases {
		html += `
			<div class="flex items-center space-x-2 p-2 hover:bg-gray-50 rounded">
				<input type="checkbox" name="database_ids" value="` + db.ID.String() + `" class="checkbox checkbox-primary">
				<div class="flex-1">
					<div class="font-medium">` + db.Name + `</div>
					<div class="text-sm text-gray-600">PostgreSQL ` + db.PgVersion + `</div>
				</div>
			</div>
		`
	}
	html += `</div>`

	return c.HTML(http.StatusOK, html)
}

func (h *handlers) bulkCreateBackupDestinationListHandler(c echo.Context) error {
	ctx := c.Request().Context()

	destinations, err := h.servs.DestinationsService.GetAllDestinations(ctx)
	if err != nil {
		return c.HTML(http.StatusInternalServerError, `<option value="">Error loading destinations</option>`)
	}

	html := `<option value="">Select a destination</option>`
	for _, dest := range destinations {
		html += `<option value="` + dest.ID.String() + `">` + dest.Name + `</option>`
	}

	return c.HTML(http.StatusOK, html)
}

func bulkCreateBackupButton() nodx.Node {
	htmxAttributes := func(url string) nodx.Node {
		return nodx.Group(
			htmx.HxPost(url),
			htmx.HxInclude("#bulk-create-backup-form"),
			htmx.HxDisabledELT(".bulk-create-backup-btn"),
			htmx.HxIndicator("#bulk-create-backup-loading"),
			htmx.HxValidate("true"),
		)
	}

	mo := component.Modal(component.ModalParams{
		Size:  component.SizeLg,
		Title: "Bulk Create Backup Tasks",
		Content: []nodx.Node{
			nodx.FormEl(
				nodx.Id("bulk-create-backup-form"),
				nodx.Class("space-y-4"),
				htmx.HxPost("/dashboard/backups/bulk-create"),

				alpine.XData(`{
					is_local: "true",
				}`),

				nodx.Div(
					nodx.Class("alert alert-info"),
					nodx.Div(
						nodx.Class("flex"),
						lucide.Info(nodx.Class("w-4 h-4 mr-2 mt-0.5")),
						nodx.Div(
							nodx.P(nodx.Class("font-medium"), component.SpanText("Create backup tasks for multiple databases")),
							nodx.P(nodx.Class("text-sm"), component.SpanText("Select databases below and configure backup settings. All selected databases will use the same configuration.")),
						),
					),
				),

				// Database selection
				nodx.Div(
					nodx.Class("space-y-2"),
					nodx.Div(
						nodx.Class("flex justify-between items-center"),
						nodx.Div(
							nodx.Class("font-medium"),
							component.SpanText("Select Databases:"),
						),
						nodx.Div(
							nodx.Class("flex space-x-2"),
							nodx.Button(
								nodx.Type("button"),
								nodx.Class("btn btn-xs btn-outline"),
								alpine.XOn("click", "document.querySelectorAll('#bulk-database-list input[type=checkbox]').forEach(cb => cb.checked = true)"),
								component.SpanText("Select All"),
							),
							nodx.Button(
								nodx.Type("button"),
								nodx.Class("btn btn-xs btn-outline"),
								alpine.XOn("click", "document.querySelectorAll('#bulk-database-list input[type=checkbox]').forEach(cb => cb.checked = false)"),
								component.SpanText("Deselect All"),
							),
						),
					),
					nodx.Div(
						nodx.Id("bulk-database-list"),
						htmx.HxGet("/dashboard/backups/bulk-create/databases"),
						htmx.HxTrigger("load"),
						component.SpanText("Loading databases..."),
					),
				),

				// Backup configuration
				nodx.Div(
					nodx.Class("grid grid-cols-1 md:grid-cols-2 gap-4"),

					component.InputControl(component.InputControlParams{
						Name:        "name_template",
						Label:       "Name Template",
						Placeholder: "{database} Backup",
						Required:    true,
						Type:        component.InputTypeText,
						HelpText:    "Template for backup names. Use {database} or {db} as placeholder for database name.",
					}),

					component.InputControl(component.InputControlParams{
						Name:        "cron_expression",
						Label:       "Cron Expression",
						Placeholder: "0 2 * * *",
						Required:    true,
						Type:        component.InputTypeText,
						HelpText:    "Schedule for backup execution (e.g., '0 2 * * *' for daily at 2 AM).",
					}),

					component.SelectControl(component.SelectControlParams{
						Name:        "time_zone",
						Label:       "Time Zone",
						Placeholder: "Select timezone",
						Required:    true,
						HelpText:    "Timezone for the cron schedule",
						Children: []nodx.Node{
							timezoneSelectOptions(),
						},
					}),

					component.InputControl(component.InputControlParams{
						Name:        "dest_dir",
						Label:       "Destination Directory",
						Placeholder: "/backups",
						Required:    true,
						Type:        component.InputTypeText,
						HelpText:    "Directory where backups will be stored",
					}),

					component.InputControl(component.InputControlParams{
						Name:        "retention_days",
						Label:       "Retention Days",
						Placeholder: "30",
						Required:    true,
						Type:        component.InputTypeNumber,
						HelpText:    "Number of days to keep backups",
					}),
				),

				// Storage options
				nodx.Div(
					nodx.Class("space-y-2"),
					nodx.Div(
						nodx.Class("font-medium"),
						component.SpanText("Storage Options:"),
					),
					nodx.Div(
						nodx.Class("flex items-center space-x-4"),
						nodx.Div(
							nodx.Class("flex items-center space-x-2"),
							nodx.Input(
								nodx.Type("radio"),
								nodx.Name("is_local"),
								nodx.Value("true"),
								nodx.Class("radio radio-primary"),
								alpine.XModel("is_local"),
								nodx.Checked("checked"),
							),
							component.SpanText("Local Storage"),
						),
						nodx.Div(
							nodx.Class("flex items-center space-x-2"),
							nodx.Input(
								nodx.Type("radio"),
								nodx.Name("is_local"),
								nodx.Value("false"),
								nodx.Class("radio radio-primary"),
								alpine.XModel("is_local"),
							),
							component.SpanText("Remote Storage"),
						),
					),
					nodx.Div(
						nodx.Class("mt-2"),
						alpine.Template(
							alpine.XIf("is_local == 'false'"),
							nodx.Div(
								nodx.Class("form-control"),
								nodx.LabelEl(
									nodx.Class("label"),
									nodx.SpanEl(
										nodx.Class("label-text"),
										component.SpanText("Destination"),
									),
								),
								nodx.Select(
									nodx.Name("destination_id"),
									nodx.Class("select select-bordered w-full"),
									nodx.Id("destination-select"),
									htmx.HxGet("/dashboard/backups/bulk-create/destinations"),
									htmx.HxTrigger("intersect once"),
									nodx.Option(
										nodx.Value(""),
										component.SpanText("Select a destination"),
									),
								),
							),
						),
					),
				),

				// Backup options
				nodx.Div(
					nodx.Class("space-y-2"),
					nodx.Div(
						nodx.Class("font-medium"),
						component.SpanText("Backup Options:"),
					),
					nodx.Div(
						nodx.Class("grid grid-cols-2 gap-2"),
						checkboxOption("is_active", "Active", "Start backup tasks immediately", true),
						checkboxOption("opt_data_only", "Data Only", "Backup only data, not schema", false),
						checkboxOption("opt_schema_only", "Schema Only", "Backup only schema, not data", false),
						checkboxOption("opt_clean", "Clean", "Drop objects before recreation", false),
						checkboxOption("opt_if_exists", "If Exists", "Use IF EXISTS for drops", false),
						checkboxOption("opt_create", "Create", "Include database creation", false),
						checkboxOption("opt_no_comments", "No Comments", "Exclude comments from backup", false),
					),
				),

				nodx.Div(
					nodx.Class("flex justify-end pt-4"),
					nodx.Div(
						nodx.Class("flex items-center space-x-2"),
						component.HxLoadingMd("bulk-create-backup-loading"),
						nodx.Button(
							htmxAttributes("/dashboard/backups/bulk-create"),
							nodx.Class("bulk-create-backup-btn btn btn-primary"),
							nodx.Type("button"),
							component.SpanText("Create Backup Tasks"),
							lucide.Save(),
						),
					),
				),
			),
		},
	})

	button := nodx.Button(
		mo.OpenerAttr,
		nodx.Class("btn btn-secondary"),
		component.SpanText("Bulk Create"),
		lucide.Plus(),
	)

	return nodx.Div(
		nodx.Class("inline-block"),
		mo.HTML,
		button,
	)
}

func checkboxOption(name, label, helpText string, defaultChecked bool) nodx.Node {
	return nodx.Div(
		nodx.Class("flex items-start space-x-2"),
		nodx.Input(
			nodx.Type("checkbox"),
			nodx.Name(name),
			nodx.Value("true"),
			nodx.Class("checkbox checkbox-primary mt-1"),
			func() nodx.Node {
				if defaultChecked {
					return nodx.Checked("checked")
				}
				return nodx.Group()
			}(),
		),
		nodx.Div(
			nodx.Div(
				nodx.Class("font-medium text-sm"),
				component.SpanText(label),
			),
			nodx.Div(
				nodx.Class("text-xs text-gray-600"),
				component.SpanText(helpText),
			),
		),
	)
}

func timezoneSelectOptions() nodx.Node {
	var options []nodx.Node

	for _, tz := range staticdata.Timezones {
		options = append(options, nodx.Option(
			nodx.Value(tz.TzCode),
			component.SpanText(tz.Label),
		))
	}

	return nodx.Group(options...)
}
