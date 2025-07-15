package destinations

import (
	"database/sql"

	"github.com/eduardolat/pgbackweb/internal/database/dbgen"
	"github.com/eduardolat/pgbackweb/internal/validate"
	"github.com/eduardolat/pgbackweb/internal/view/web/component"
	"github.com/eduardolat/pgbackweb/internal/view/web/respondhtmx"
	"github.com/labstack/echo/v4"
	nodx "github.com/nodxdev/nodxgo"
	htmx "github.com/nodxdev/nodxgo-htmx"
	lucide "github.com/nodxdev/nodxgo-lucide"
)

type createDestinationDTO struct {
	Name                string `form:"name" validate:"required"`
	BucketName          string `form:"bucket_name" validate:"required"`
	AccessKey           string `form:"access_key" validate:"required"`
	SecretKey           string `form:"secret_key" validate:"required"`
	Region              string `form:"region" validate:"required"`
	Endpoint            string `form:"endpoint" validate:"required"`
	EncryptionType      string `form:"encryption_type"`
	EncryptionKeyID     string `form:"encryption_key_id"`
	EncryptionKeyRegion string `form:"encryption_key_region"`
}

func (h *handlers) createDestinationHandler(c echo.Context) error {
	ctx := c.Request().Context()

	var formData createDestinationDTO
	if err := c.Bind(&formData); err != nil {
		return respondhtmx.ToastError(c, err.Error())
	}
	if err := validate.Struct(&formData); err != nil {
		return respondhtmx.ToastError(c, err.Error())
	}

	_, err := h.servs.DestinationsService.CreateDestination(
		ctx, dbgen.DestinationsServiceCreateDestinationParams{
			Name:                formData.Name,
			AccessKey:           formData.AccessKey,
			SecretKey:           formData.SecretKey,
			Region:              formData.Region,
			Endpoint:            formData.Endpoint,
			BucketName:          formData.BucketName,
			EncryptionType:      formData.EncryptionType,
			EncryptionKeyID:     sql.NullString{Valid: formData.EncryptionKeyID != "", String: formData.EncryptionKeyID},
			EncryptionKeyRegion: sql.NullString{Valid: formData.EncryptionKeyRegion != "", String: formData.EncryptionKeyRegion},
		},
	)
	if err != nil {
		return respondhtmx.ToastError(c, err.Error())
	}

	return respondhtmx.Redirect(c, "/dashboard/destinations")
}

func createDestinationButton() nodx.Node {
	htmxAttributes := func(url string) nodx.Node {
		return nodx.Group(
			htmx.HxPost(url),
			htmx.HxInclude("#add-destination-form"),
			htmx.HxDisabledELT(".add-destination-btn"),
			htmx.HxIndicator("#add-destination-loading"),
			htmx.HxValidate("true"),
		)
	}

	mo := component.Modal(component.ModalParams{
		Size:  component.SizeMd,
		Title: "Add destination",
		Content: []nodx.Node{
			nodx.FormEl(
				nodx.Id("add-destination-form"),
				nodx.Class("space-y-2"),

				component.InputControl(component.InputControlParams{
					Name:        "name",
					Label:       "Name",
					Placeholder: "My destination",
					Required:    true,
					Type:        component.InputTypeText,
					HelpText:    "A name to easily identify the destination",
				}),

				component.InputControl(component.InputControlParams{
					Name:        "bucket_name",
					Label:       "Bucket name",
					Placeholder: "my-bucket",
					Required:    true,
					Type:        component.InputTypeText,
				}),

				component.InputControl(component.InputControlParams{
					Name:        "endpoint",
					Label:       "Endpoint",
					Placeholder: "s3-us-west-1.amazonaws.com",
					Required:    true,
					Type:        component.InputTypeText,
				}),

				component.InputControl(component.InputControlParams{
					Name:        "region",
					Label:       "Region",
					Placeholder: "us-west-1",
					Required:    true,
					Type:        component.InputTypeText,
				}),

				component.InputControl(component.InputControlParams{
					Name:        "access_key",
					Label:       "Access key",
					Placeholder: "Access key",
					Required:    true,
					Type:        component.InputTypeText,
					HelpText:    "It will be stored securely using PGP encryption.",
				}),

				component.InputControl(component.InputControlParams{
					Name:        "secret_key",
					Label:       "Secret key",
					Placeholder: "Secret key",
					Required:    true,
					Type:        component.InputTypeText,
					HelpText:    "It will be stored securely using PGP encryption.",
				}),

				// Encryption settings
				nodx.Div(
					nodx.Class("border-t pt-4 mt-4"),
					nodx.H3(
						nodx.Class("text-lg font-semibold mb-2"),
						nodx.Text("Encryption Settings"),
					),
					
					component.SelectControl(component.SelectControlParams{
						Name:        "encryption_type",
						Label:       "Encryption Type",
						Required:    false,
						HelpText:    "Choose the server-side encryption method for your S3 objects.",
						Children: []nodx.Node{
							nodx.Option(
								nodx.Value("none"),
								nodx.Selected(""),
								nodx.Text("No encryption"),
							),
							nodx.Option(
								nodx.Value("aes256"),
								nodx.Text("AES-256 (S3 managed)"),
							),
							nodx.Option(
								nodx.Value("aws:kms"),
								nodx.Text("AWS KMS"),
							),
							nodx.Option(
								nodx.Value("sse-c"),
								nodx.Text("SSE-C (Customer-provided key)"),
							),
						},
					}),

					component.InputControl(component.InputControlParams{
						Name:        "encryption_key_id",
						Label:       "Encryption Key",
						Placeholder: "Base64-encoded 256-bit key for SSE-C, or KMS key ARN for AWS KMS",
						Required:    false,
						Type:        component.InputTypeText,
						HelpText:    "For SSE-C: base64-encoded 256-bit encryption key. For AWS KMS: KMS Key ID or ARN.",
					}),

					component.InputControl(component.InputControlParams{
						Name:        "encryption_key_region",
						Label:       "KMS Key Region",
						Placeholder: "us-east-1",
						Required:    false,
						Type:        component.InputTypeText,
						HelpText:    "Region where the KMS key is located (optional, defaults to bucket region).",
					}),
				),
			),

			nodx.Div(
				nodx.Class("flex justify-between items-center pt-4"),
				nodx.Div(
					nodx.Button(
						htmxAttributes("/dashboard/destinations/test"),
						nodx.Class("add-destination-btn btn btn-neutral btn-outline"),
						nodx.Type("button"),
						component.SpanText("Test connection"),
						lucide.PlugZap(),
					),
				),
				nodx.Div(
					nodx.Class("flex justify-end items-center space-x-2"),
					component.HxLoadingMd("add-destination-loading"),
					nodx.Button(
						htmxAttributes("/dashboard/destinations"),
						nodx.Class("add-destination-btn btn btn-primary"),
						nodx.Type("button"),
						component.SpanText("Add destination"),
						lucide.Save(),
					),
				),
			),
		},
	})

	button := nodx.Button(
		mo.OpenerAttr,
		nodx.Class("btn btn-primary"),
		component.SpanText("Add destination"),
		lucide.Plus(),
	)

	return nodx.Div(
		nodx.Class("inline-block"),
		mo.HTML,
		button,
	)
}
