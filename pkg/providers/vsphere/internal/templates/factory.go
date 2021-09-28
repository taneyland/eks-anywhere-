package templates

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aws/eks-anywhere/pkg/api/v1alpha1"
	"github.com/aws/eks-anywhere/pkg/logger"
	"github.com/aws/eks-anywhere/pkg/providers/vsphere/internal/tags"
)

type Factory struct {
	client          GovcClient
	datastore       string
	resourcePool    string
	templateLibrary string
	tagsFactory     *tags.Factory
}

type GovcClient interface {
	CreateLibrary(ctx context.Context, datastore, library string) error
	DeployTemplateFromLibrary(ctx context.Context, templateDir, templateName, library, resourcePool string, resizeDisk2 bool) error
	SearchTemplate(ctx context.Context, datacenter string, machineConfig *v1alpha1.VSphereMachineConfig) (string, error)
	ImportTemplate(ctx context.Context, library, ovaURL, name string) error
	LibraryElementExists(ctx context.Context, library string) (bool, error)
	DeleteLibraryElement(ctx context.Context, library string) error
	ListTags(ctx context.Context) ([]string, error)
	CreateTag(ctx context.Context, tag, category string) error
	AddTag(ctx context.Context, path, tag string) error
	ListCategories(ctx context.Context) ([]string, error)
	CreateCategoryForVM(ctx context.Context, name string) error
}

func NewFactory(client GovcClient, datastore, resourcePool, templateLibrary string) *Factory {
	return &Factory{
		client:          client,
		datastore:       datastore,
		resourcePool:    resourcePool,
		templateLibrary: templateLibrary,
		tagsFactory:     tags.NewFactory(client),
	}
}

func (f *Factory) CreateIfMissing(ctx context.Context, datacenter string, machineConfig *v1alpha1.VSphereMachineConfig, ovaURL string, tagsByCategory map[string][]string) error {
	templateFullPath, err := f.client.SearchTemplate(ctx, datacenter, machineConfig)
	if err != nil {
		return fmt.Errorf("error checking for template: %v", err)
	}
	if err == nil && len(templateFullPath) > 0 {
		machineConfig.Spec.Template = templateFullPath
		logger.V(2).Info("Template already exists. Skipping creation", "template", machineConfig.Spec.Template)
		return nil
	}

	logger.V(2).Info("Template not available. Creating", "template", machineConfig.Spec.Template)

	osFamily := machineConfig.Spec.OSFamily
	if err = f.createTemplate(ctx, machineConfig.Spec.Template, ovaURL, string(osFamily)); err != nil {
		return err
	}

	if err = f.tagsFactory.TagTemplate(ctx, machineConfig.Spec.Template, tagsByCategory); err != nil {
		return err
	}
	return nil
}

func (f *Factory) createTemplate(ctx context.Context, templatePath, ovaURL, osFamily string) error {
	if err := f.createLibraryIfMissing(ctx); err != nil {
		return err
	}

	logger.Info("Creating template. This might take a while.") // TODO: add rough estimate timing?
	templateName := filepath.Base(templatePath)
	templateDir := filepath.Dir(templatePath)

	if err := f.importOVAIfMissing(ctx, templateName, ovaURL); err != nil {
		return err
	}

	var resizeDisk2 bool
	if strings.EqualFold(osFamily, string(v1alpha1.Bottlerocket)) {
		resizeDisk2 = true
	}
	if err := f.client.DeployTemplateFromLibrary(ctx, templateDir, templateName, f.templateLibrary, f.resourcePool, resizeDisk2); err != nil {
		return fmt.Errorf("failed deploying template: %v", err)
	}

	return nil
}

func (f *Factory) createLibraryIfMissing(ctx context.Context) error {
	libraryExists, err := f.client.LibraryElementExists(ctx, f.templateLibrary)
	if err != nil {
		return fmt.Errorf("failed to validate library for new template: %v", err)
	}

	if !libraryExists {
		logger.V(2).Info("Creating library", "library", f.templateLibrary)
		if err = f.client.CreateLibrary(ctx, f.datastore, f.templateLibrary); err != nil {
			return fmt.Errorf("failed creating library for new template: %v", err)
		}
	}

	return nil
}

func (f *Factory) importOVAIfMissing(ctx context.Context, templateName, ovaURL string) error {
	templateExistsInLibrary, err := f.client.LibraryElementExists(ctx, filepath.Join(f.templateLibrary, templateName))
	if err != nil {
		return fmt.Errorf("failed to validate template in library for new template: %v", err)
	}

	if templateExistsInLibrary {
		err = f.client.DeleteLibraryElement(ctx, filepath.Join(f.templateLibrary, templateName))
	}

	logger.V(2).Info("Importing template from ova url", "ova", ovaURL)
	if err = f.client.ImportTemplate(ctx, f.templateLibrary, ovaURL, templateName); err != nil {
		return fmt.Errorf("failed importing template into library: %v", err)
	}

	return nil
}
