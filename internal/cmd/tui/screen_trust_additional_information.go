// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/policy"
)

type trustAdditionalInformationScreen struct {
	operationList list.Model
}

func (s *trustAdditionalInformationScreen) Update(msg tea.Msg, m *model) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	s.operationList, cmd = s.operationList.Update(msg)
	return *m, cmd
}

func (s *trustAdditionalInformationScreen) View(m *model) string {
	return m.renderScreen("Home › Trust › Additional Information", s.operationList.View(), renderActionHints(true))
}

func trustAdditionalInformationMenuItems() []list.Item {
	return []list.Item{
		item{title: "Schema Version", desc: "Loading root metadata..."},
		item{title: "Repository Location", desc: "Loading root metadata..."},
	}
}

func (s *trustAdditionalInformationScreen) refreshRootMetadata(m *model) {
	repo := m.repo
	if repo == nil {
		var err error
		repo, err = gittuf.LoadRepository(".")
		if err != nil {
			s.setMetadataError(err)
			return
		}
	}

	targetRef := policy.PolicyRef
	if m.options != nil && m.options.targetRef != "" {
		targetRef = m.options.targetRef
	}

	if !strings.HasPrefix(targetRef, "refs/gittuf/") {
		targetRef = "refs/gittuf/" + targetRef
	}

	state, err := policy.LoadCurrentState(m.ctx, repo.GetStorer(), targetRef)
	if err != nil {
		s.setMetadataError(err)
		return
	}
	rootMetadata, err := state.GetRootMetadata(false)
	if err != nil {
		s.setMetadataError(err)
		return
	}

	repositoryLocation := rootMetadata.GetRepositoryLocation()
	if repositoryLocation == "" {
		repositoryLocation = "Not set"
	}
	items := trustAdditionalInformationMenuItems()
	items[0] = item{title: "Schema Version", desc: rootMetadata.GetSchemaVersion()}
	items[1] = item{title: "Repository Location", desc: repositoryLocation}
	s.operationList.SetItems(items)
}

func (s *trustAdditionalInformationScreen) setMetadataError(err error) {
	items := trustAdditionalInformationMenuItems()
	items[0] = item{title: "Schema Version", desc: err.Error()}
	items[1] = item{title: "Repository Location", desc: err.Error()}
	s.operationList.SetItems(items)
}
