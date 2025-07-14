package databases

import (
	"database/sql"
	"net/http"

	"github.com/eduardolat/pgbackweb/internal/service/databases"
	"github.com/eduardolat/pgbackweb/internal/validate"
	"github.com/eduardolat/pgbackweb/internal/view/web/component"
	"github.com/eduardolat/pgbackweb/internal/view/web/respondhtmx"
	"github.com/labstack/echo/v4"
	nodx "github.com/nodxdev/nodxgo"
	htmx "github.com/nodxdev/nodxgo-htmx"
	lucide "github.com/nodxdev/nodxgo-lucide"
)

type discoverDatabasesDTO struct {
	Version          string `form:"version" validate:"required"`
	ConnectionString string `form:"connection_string" validate:"required"`
}

type bulkImportDTO struct {
	Version           string   `form:"version" validate:"required"`
	ConnectionString  string   `form:"connection_string" validate:"required"`
	SelectedDatabases []string `form:"selected_databases"`
}

type bulkUpdateConnectionStringDTO struct {
	DatabaseIDs         []string `form:"database_ids"`
	NewConnectionString string   `form:"new_connection_string" validate:"required"`
	UpdateMode          string   `form:"update_mode"`
}

func (h *handlers) discoverDatabasesHandler(c echo.Context) error {
	ctx := c.Request().Context()

	var formData discoverDatabasesDTO
	if err := c.Bind(&formData); err != nil {
		return respondhtmx.ToastError(c, err.Error())
	}
	if err := validate.Struct(&formData); err != nil {
		return respondhtmx.ToastError(c, err.Error())
	}

	discovered, err := h.servs.DatabasesService.DiscoverDatabases(
		ctx, formData.Version, formData.ConnectionString,
	)
	if err != nil {
		return respondhtmx.ToastError(c, err.Error())
	}

	return c.HTML(http.StatusOK, databasesTable(discovered, formData.Version, formData.ConnectionString))
}

func (h *handlers) bulkImportDatabasesHandler(c echo.Context) error {
	ctx := c.Request().Context()

	var formData bulkImportDTO
	if err := c.Bind(&formData); err != nil {
		return respondhtmx.ToastError(c, err.Error())
	}
	if err := validate.Struct(&formData); err != nil {
		return respondhtmx.ToastError(c, err.Error())
	}

	if len(formData.SelectedDatabases) == 0 {
		return respondhtmx.ToastError(c, "Please select at least one database to import")
	}

	req := databases.BulkImportRequest{
		ConnectionString:  formData.ConnectionString,
		Version:          formData.Version,
		SelectedDatabases: formData.SelectedDatabases,
	}

	err := h.servs.DatabasesService.BulkImportDatabases(ctx, req)
	if err != nil {
		return respondhtmx.ToastError(c, err.Error())
	}

	return respondhtmx.Redirect(c, "/dashboard/databases")
}

func (h *handlers) bulkUpdateConnectionStringHandler(c echo.Context) error {
	ctx := c.Request().Context()

	var formData bulkUpdateConnectionStringDTO
	if err := c.Bind(&formData); err != nil {
		return respondhtmx.ToastError(c, err.Error())
	}
	if err := validate.Struct(&formData); err != nil {
		return respondhtmx.ToastError(c, err.Error())
	}

	if len(formData.DatabaseIDs) == 0 {
		return respondhtmx.ToastError(c, "Please select at least one database to update")
	}

	req := databases.BulkUpdateConnectionStringRequest{
		DatabaseIDs:           formData.DatabaseIDs,
		NewConnectionString:   formData.NewConnectionString,
		OnlyUpdateHost:        formData.UpdateMode == "host",
		OnlyUpdateCredentials: formData.UpdateMode == "credentials",
	}

	err := h.servs.DatabasesService.BulkUpdateConnectionString(ctx, req)
	if err != nil {
		return respondhtmx.ToastError(c, err.Error())
	}

	return respondhtmx.Redirect(c, "/dashboard/databases")
}

func (h *handlers) bulkUpdateDatabaseListHandler(c echo.Context) error {
	ctx := c.Request().Context()

	databases, err := h.servs.DatabasesService.GetAllDatabases(ctx)
	if err != nil {
		return c.HTML(http.StatusInternalServerError, `<div class="alert alert-error">Error loading databases</div>`)
	}

	if len(databases) == 0 {
		return c.HTML(http.StatusOK, `<div class="text-center py-4">No databases found</div>`)
	}

	html := `<div class="space-y-2">`
	for _, db := range databases {
		html += `
			<div class="flex items-center space-x-2">
				<input type="checkbox" name="database_ids" value="` + db.ID.String() + `" class="checkbox checkbox-primary">
				<div class="flex-1">
					<div class="font-medium">` + db.Name + `</div>
					<div class="text-sm text-gray-600">Version: ` + db.PgVersion + `</div>
				</div>
			</div>
		`
	}
	html += `</div>`

	return c.HTML(http.StatusOK, html)
}

func databasesTable(databases []databases.DatabaseDiscovery, version, connectionString string) string {
	if len(databases) == 0 {
		return `<div class="text-center py-8">
			<p class="text-gray-500">No databases found</p>
		</div>`
	}

	rows := ""
	for _, db := range databases {
		rows += `
			<tr>
				<td>
					<input type="checkbox" name="selected_databases" value="` + db.Name + `" class="checkbox checkbox-primary">
				</td>
				<td class="font-medium">` + db.Name + `</td>
				<td>` + db.Size + `</td>
				<td>` + db.Owner + `</td>
				<td>` + db.Comment + `</td>
			</tr>
		`
	}

	return `
		<div class="space-y-4">
			<div class="flex justify-between items-center">
				<h3 class="text-lg font-medium">Discovered Databases</h3>
				<div class="flex items-center space-x-2">
					<button type="button" onclick="checkAll()" class="btn btn-sm btn-outline">Select All</button>
					<button type="button" onclick="uncheckAll()" class="btn btn-sm btn-outline">Select None</button>
				</div>
			</div>
			<div class="overflow-x-auto">
				<table class="table w-full">
					<thead>
						<tr>
							<th class="w-1"></th>
							<th>Name</th>
							<th>Size</th>
							<th>Owner</th>
							<th>Comment</th>
						</tr>
					</thead>
					<tbody>
						` + rows + `
					</tbody>
				</table>
			</div>
			<input type="hidden" name="version" value="` + version + `">
			<input type="hidden" name="connection_string" value="` + connectionString + `">
			<div class="flex justify-end">
				<button type="submit" class="btn btn-primary">
					<span>Import Selected Databases</span>
					<svg class="w-4 h-4 ml-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12"></path>
					</svg>
				</button>
			</div>
		</div>
		<script>
			function checkAll() {
				const checkboxes = document.querySelectorAll('input[name="selected_databases"]');
				checkboxes.forEach(cb => cb.checked = true);
			}
			function uncheckAll() {
				const checkboxes = document.querySelectorAll('input[name="selected_databases"]');
				checkboxes.forEach(cb => cb.checked = false);
			}
		</script>
	`
}

func bulkImportDatabaseButton() nodx.Node {
	htmxAttributes := func(url string) nodx.Node {
		return nodx.Group(
			htmx.HxPost(url),
			htmx.HxInclude("#bulk-import-form"),
			htmx.HxDisabledELT(".bulk-import-btn"),
			htmx.HxIndicator("#bulk-import-loading"),
			htmx.HxValidate("true"),
		)
	}

	mo := component.Modal(component.ModalParams{
		Size:  component.SizeLg,
		Title: "Bulk Import Databases",
		Content: []nodx.Node{
			nodx.FormEl(
				nodx.Id("bulk-import-form"),
				nodx.Class("space-y-4"),
				htmx.HxPost("/dashboard/databases/bulk-import"),

				nodx.Div(
					nodx.Class("space-y-2"),
					component.SelectControl(component.SelectControlParams{
						Name:        "version",
						Label:       "PostgreSQL Version",
						Placeholder: "Select a version",
						Required:    true,
						HelpText:    "The version of PostgreSQL server to connect to",
						Children: []nodx.Node{
							component.PGVersionSelectOptions(sql.NullString{}),
						},
					}),

					component.InputControl(component.InputControlParams{
						Name:        "connection_string",
						Label:       "Connection String",
						Placeholder: "postgresql://user:password@localhost:5432/",
						Required:    true,
						Type:        component.InputTypeText,
						HelpText:    "Base connection string without specific database name. We'll query all databases from this server.",
					}),
				),

				nodx.Div(
					nodx.Class("flex justify-between items-center pt-4"),
					nodx.Button(
						htmxAttributes("/dashboard/databases/bulk-import/discover"),
						htmx.HxTarget("#discovered-databases"),
						nodx.Class("bulk-import-btn btn btn-outline"),
						nodx.Type("button"),
						component.SpanText("Discover Databases"),
						lucide.Search(),
					),
					component.HxLoadingMd("bulk-import-loading"),
				),

				nodx.Div(
					nodx.Id("discovered-databases"),
					nodx.Class("mt-4"),
				),
			),
		},
	})

	button := nodx.Button(
		mo.OpenerAttr,
		nodx.Class("btn btn-secondary"),
		component.SpanText("Bulk Import"),
		lucide.Database(),
	)

	return nodx.Div(
		nodx.Class("inline-block"),
		mo.HTML,
		button,
	)
}

func bulkUpdateConnectionStringButton() nodx.Node {
	htmxAttributes := func(url string) nodx.Node {
		return nodx.Group(
			htmx.HxPost(url),
			htmx.HxInclude("#bulk-update-form"),
			htmx.HxDisabledELT(".bulk-update-btn"),
			htmx.HxIndicator("#bulk-update-loading"),
			htmx.HxValidate("true"),
		)
	}

	mo := component.Modal(component.ModalParams{
		Size:  component.SizeLg,
		Title: "Bulk Update Connection Strings",
		Content: []nodx.Node{
			nodx.FormEl(
				nodx.Id("bulk-update-form"),
				nodx.Class("space-y-4"),
				htmx.HxPost("/dashboard/databases/bulk-update"),

				nodx.Div(
					nodx.Class("alert alert-info"),
					nodx.Div(
						nodx.Class("flex"),
						lucide.Info(nodx.Class("w-4 h-4 mr-2 mt-0.5")),
						nodx.Div(
							nodx.P(nodx.Class("font-medium"), component.SpanText("Select databases to update")),
							nodx.P(nodx.Class("text-sm"), component.SpanText("Choose databases from the list below, then specify the new connection string or update mode.")),
						),
					),
				),

				nodx.Div(
					nodx.Id("database-selection"),
					nodx.Class("space-y-2"),
					nodx.Div(
						nodx.Class("font-medium"),
						component.SpanText("Select databases:"),
					),
					nodx.Div(
						nodx.Id("database-list"),
						htmx.HxGet("/dashboard/databases/bulk-update/list"),
						htmx.HxTrigger("load"),
						component.SpanText("Loading databases..."),
					),
				),

				nodx.Div(
					nodx.Class("space-y-2"),
					component.InputControl(component.InputControlParams{
						Name:        "new_connection_string",
						Label:       "New Connection String",
						Placeholder: "postgresql://user:password@localhost:5432/",
						Required:    true,
						Type:        component.InputTypeText,
						HelpText:    "The new connection string. For partial updates, only the specified parts will be used.",
					}),
				),

				nodx.Div(
					nodx.Class("space-y-2"),
					nodx.Div(
						nodx.Class("font-medium"),
						component.SpanText("Update mode:"),
					),
					nodx.Div(
						nodx.Class("space-y-2"),
						nodx.Div(
							nodx.Class("flex items-center space-x-2"),
							nodx.Input(
								nodx.Type("radio"),
								nodx.Name("update_mode"),
								nodx.Value("full"),
								nodx.Class("radio radio-primary"),
								nodx.Checked("checked"),
							),
							component.SpanText("Full replacement - Replace entire connection string"),
						),
						nodx.Div(
							nodx.Class("flex items-center space-x-2"),
							nodx.Input(
								nodx.Type("radio"),
								nodx.Name("update_mode"),
								nodx.Value("host"),
								nodx.Class("radio radio-primary"),
							),
							component.SpanText("Host only - Update only host and port"),
						),
						nodx.Div(
							nodx.Class("flex items-center space-x-2"),
							nodx.Input(
								nodx.Type("radio"),
								nodx.Name("update_mode"),
								nodx.Value("credentials"),
								nodx.Class("radio radio-primary"),
							),
							component.SpanText("Credentials only - Update only username and password"),
						),
					),
				),

				nodx.Div(
					nodx.Class("flex justify-end pt-4"),
					nodx.Div(
						nodx.Class("flex items-center space-x-2"),
						component.HxLoadingMd("bulk-update-loading"),
						nodx.Button(
							htmxAttributes("/dashboard/databases/bulk-update"),
							nodx.Class("bulk-update-btn btn btn-primary"),
							nodx.Type("button"),
							component.SpanText("Update Connection Strings"),
							lucide.Save(),
						),
					),
				),
			),
		},
	})

	button := nodx.Button(
		mo.OpenerAttr,
		nodx.Class("btn btn-outline"),
		component.SpanText("Bulk Update"),
		lucide.Save(),
	)

	return nodx.Div(
		nodx.Class("inline-block"),
		mo.HTML,
		button,
	)
}
