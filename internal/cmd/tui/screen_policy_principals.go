// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
)

type policyPrincipalsScreen struct {
	principals    []tuf.Principal
	list          list.Model
	confirmDelete bool
	deleteTarget  string
}

type policyPrincipalsFormScreen struct {
	inputs     []textinput.Model
	focusIndex int
	action     string // "Add Person", "Edit Person"
}

func (s *policyPrincipalsScreen) refreshPrincipals(ctx context.Context, o *options) {
	s.principals = getCurrPrincipals(ctx, o)
	s.updatePrincipalsList()
}

func (s *policyPrincipalsScreen) updatePrincipalsList() {
	items := make([]list.Item, len(s.principals))
	for i, p := range s.principals {
		var desc strings.Builder
		keys := p.Keys()
		if len(keys) > 0 {
			desc.WriteString("Keys: ")
			for j, k := range keys {
				if j > 0 {
					desc.WriteString(", ")
				}
				desc.WriteString(k.KeyID)
			}
		} else {
			desc.WriteString("Keys: None")
		}

		if person, ok := p.(*tufv02.Person); ok {
			if len(person.AssociatedIdentities) > 0 {
				desc.WriteString("\nIdentities: ")
				count := 0
				for provider, id := range person.AssociatedIdentities {
					if count > 0 {
						desc.WriteString(", ")
					}
					fmt.Fprintf(&desc, "%s::%s", provider, id)
					count++
				}
			}
		}

		if len(p.CustomMetadata()) > 0 {
			desc.WriteString("\nCustom: ")
			count := 0
			for k, v := range p.CustomMetadata() {
				if count > 0 {
					desc.WriteString(", ")
				}
				fmt.Fprintf(&desc, "%s=%s", k, v)
				count++
			}
		}

		items[i] = item{title: p.ID(), desc: desc.String()}
	}
	s.list.SetItems(items)
}

func (f *policyPrincipalsFormScreen) initInputs(action string) {
	f.action = action
	f.inputs = initInputs([]inputField{
		{"Enter Principal ID", "Principal ID:"},
		{"Enter Public Keys (paths or IDs, comma-separated)", "Public Keys:"},
		{"Enter Associated Identities (provider::identity, comma-separated)", "Identities:"},
		{"Enter Custom Metadata (Key=Value, comma-separated)", "Custom Metadata:"},
	})
	f.focusIndex = 0
}

func (f *policyPrincipalsFormScreen) initInputsPrefilled(p tuf.Principal) {
	f.initInputs("Edit Person")
	f.inputs[0].SetValue(p.ID())

	keys := []string{}
	for _, k := range p.Keys() {
		keys = append(keys, k.KeyID)
	}
	f.inputs[1].SetValue(strings.Join(keys, ", "))

	// Extracting Associated Identities is tricky because tuf.Principal doesn't expose it directly.
	// But it's stored in Custom Metadata for Person if we cast it, or we just leave it blank for manual edit.
	// We'll leave Identities blank or parse it if we can cast to *tufv02.Person.
	if person, ok := p.(*tufv02.Person); ok {
		identities := []string{}
		for provider, id := range person.AssociatedIdentities {
			identities = append(identities, fmt.Sprintf("%s::%s", provider, id))
		}
		f.inputs[2].SetValue(strings.Join(identities, ", "))
	}

	customs := []string{}
	for k, v := range p.CustomMetadata() {
		customs = append(customs, fmt.Sprintf("%s=%s", k, v))
	}
	f.inputs[3].SetValue(strings.Join(customs, ", "))
}

func (f *policyPrincipalsFormScreen) cycleFocus(key string) {
	if key == "up" || key == "shift+tab" {
		if f.focusIndex > 0 {
			f.focusIndex--
		} else {
			f.focusIndex = len(f.inputs) - 1
		}
	} else {
		if f.focusIndex < len(f.inputs)-1 {
			f.focusIndex++
		} else {
			f.focusIndex = 0
		}
	}

	for i := range f.inputs {
		if i == f.focusIndex {
			f.inputs[i].Focus()
			f.inputs[i].PromptStyle = focusedStyle
			f.inputs[i].TextStyle = focusedStyle
		} else {
			f.inputs[i].Blur()
			f.inputs[i].PromptStyle = blurredStyle
			f.inputs[i].TextStyle = blurredStyle
		}
	}
}

func (s *policyPrincipalsScreen) Update(msg tea.Msg, m *model) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if s.confirmDelete {
		return s.handleDeleteConfirm(msg, m)
	}

	if msg, ok := msg.(tea.KeyMsg); ok {
		if !m.readOnly {
			switch msg.String() {
			case "a":
				m.policyPrincipalsFormScreen.initInputs("Add Person")
				m.screen = screenPolicyPrincipalsForm
				return *m, nil
			case "e":
				if sel, ok := s.list.SelectedItem().(item); ok {
					for _, p := range s.principals {
						if p.ID() == sel.title {
							m.policyPrincipalsFormScreen.initInputsPrefilled(p)
							m.screen = screenPolicyPrincipalsForm
							return *m, nil
						}
					}
				}
			case "d":
				if sel, ok := s.list.SelectedItem().(item); ok {
					s.confirmDelete = true
					s.deleteTarget = sel.title
					return *m, nil
				}
			}
		}
	}
	s.list, cmd = s.list.Update(msg)
	return *m, cmd
}

func (s *policyPrincipalsScreen) handleDeleteConfirm(msg tea.Msg, m *model) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.String() == "y" {
			if err := repoRemovePrincipal(m.ctx, m.options, s.deleteTarget); err != nil {
				m.errorMsg = fmt.Sprintf("Error removing principal: %v", err)
			} else {
				m.footer = "Principal removed successfully!"
				s.refreshPrincipals(m.ctx, m.options)
			}
		}
		s.confirmDelete = false
		s.deleteTarget = ""
	}
	return *m, nil
}

func (s *policyPrincipalsScreen) View(m *model) string {
	overlay := ""
	if s.confirmDelete {
		overlay = "\n" + renderDeleteOverlay(s.deleteTarget) + "\n"
	}
	hint := ""
	if !m.readOnly {
		hint = "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color(colorSubtext)).Render(
			"Run `gittuf policy apply` to apply staged changes to the selected policy file.",
		)
	}

	listView := m.renderListOrEmpty(s.list, len(s.principals), "No principals configured")
	overlays := overlay + renderActionHints(m.readOnly) + hint

	return m.renderScreen("Home › Policy › Principals", listView, overlays)
}

func (f *policyPrincipalsFormScreen) Update(msg tea.Msg, m *model) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "enter":
			return f.handleFormSubmit(m)
		case "tab", "shift+tab", "up", "down":
			f.cycleFocus(msg.String())
			m.footer = ""
			return *m, nil
		}
	}
	f.inputs[f.focusIndex], cmd = f.inputs[f.focusIndex].Update(msg)
	return *m, cmd
}

func (f *policyPrincipalsFormScreen) handleFormSubmit(m *model) (tea.Model, tea.Cmd) {
	if f.focusIndex < len(f.inputs)-1 {
		f.cycleFocus("tab")
		return *m, nil
	}

	personID := f.inputs[0].Value()
	publicKeysRaw := splitAndTrim(f.inputs[1].Value())
	identitiesRaw := splitAndTrim(f.inputs[2].Value())
	customRaw := splitAndTrim(f.inputs[3].Value())

	if personID == "" {
		m.errorMsg = "Error: Principal ID is required"
		return *m, nil
	}

	publicKeys := map[string]*tufv02.Key{}
	for _, keyPath := range publicKeysRaw {
		if keyPath == "" {
			continue
		}
		key, err := gittuf.LoadPublicKey(keyPath)
		if err != nil {
			m.errorMsg = fmt.Sprintf("Error loading public key '%s': %v", keyPath, err)
			return *m, nil
		}
		publicKeys[key.ID()] = key.(*tufv02.Key)
	}

	associatedIdentities := map[string]string{}
	for _, idRaw := range identitiesRaw {
		if idRaw == "" {
			continue
		}
		split := strings.Split(idRaw, "::")
		if len(split) != 2 {
			m.errorMsg = fmt.Sprintf("Error: invalid format for associated identity '%s'", idRaw)
			return *m, nil
		}
		associatedIdentities[split[0]] = split[1]
	}

	custom := map[string]string{}
	for _, cRaw := range customRaw {
		if cRaw == "" {
			continue
		}
		split := strings.Split(cRaw, "=")
		if len(split) != 2 {
			m.errorMsg = fmt.Sprintf("Error: invalid format for custom metadata '%s'", cRaw)
			return *m, nil
		}
		custom[split[0]] = split[1]
	}

	person := &tufv02.Person{
		PersonID:             personID,
		PublicKeys:           publicKeys,
		AssociatedIdentities: associatedIdentities,
		Custom:               custom,
	}

	var err error
	switch f.action {
	case "Add Person":
		err = repoAddPrincipal(m.ctx, m.options, person)
	case "Edit Person":
		err = repoUpdatePrincipal(m.ctx, m.options, person)
	}

	if err != nil {
		m.errorMsg = fmt.Sprintf("Error: %v", err)
		return *m, nil
	}

	m.policyPrincipalsScreen.refreshPrincipals(m.ctx, m.options)
	if f.action == "Add Person" {
		m.footer = "Principal added successfully!"
	} else {
		m.footer = "Principal updated successfully!"
	}
	m.screen = screenPolicyPrincipals
	return *m, nil
}

func (f *policyPrincipalsFormScreen) View(m *model) string {
	breadcrumb := fmt.Sprintf("Home › Policy › Principals › %s", f.action)
	var b strings.Builder
	b.WriteString(titleStyle.Render(f.action))
	b.WriteString("\n\n")
	for _, input := range f.inputs {
		b.WriteString(input.View())
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString("Press Tab to advance, Enter to advance/submit, and Esc to go back.")
	b.WriteString("\n")
	b.WriteString(renderFooterBox(*m))
	if m.errorMsg != "" {
		b.WriteString("\n")
		b.WriteString(renderErrorMsg(m.errorMsg))
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		renderStatusBar(breadcrumb, m.readOnly, m.width),
		renderWithMargin(b.String()),
	)
}

func getCurrPrincipals(ctx context.Context, o *options) []tuf.Principal {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return nil
	}
	principalsMap, err := repo.ListPrincipals(ctx, "policy", o.policyName)
	if err != nil {
		return nil
	}
	var principals []tuf.Principal
	for _, p := range principalsMap {
		principals = append(principals, p)
	}
	return principals
}

func repoAddPrincipal(ctx context.Context, o *options, person *tufv02.Person) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}
	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}
	return repo.AddPrincipalToTargets(ctx, signer, o.policyName, []tuf.Principal{person}, true)
}

func repoUpdatePrincipal(ctx context.Context, o *options, person *tufv02.Person) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}
	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}
	return repo.UpdatePrincipalInTargets(ctx, signer, o.policyName, person, true)
}

func repoRemovePrincipal(ctx context.Context, o *options, personID string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}
	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}
	return repo.RemovePrincipalFromTargets(ctx, signer, o.policyName, personID, true)
}
