package api

import (
	"moo/internal/domain"
	"moo/internal/repo"
)

// Types exposés au frontend (générés dans wailsjs/go/models.ts).
type (
	PersonDetail    = repo.PersonDetail
	PersonPatch     = repo.PersonPatch
	NameInput       = repo.NameInput
	Impact          = repo.Impact
	EventInput      = repo.EventInput
	EventPatch      = repo.EventPatch
	PlaceInput      = repo.PlaceInput
	PlaceDTO        = repo.PlaceDTO
	ParentLinkPatch = repo.ParentLinkPatch
	UnionPatch      = repo.UnionPatch
	SosaEntry       = repo.SosaEntry
)

// DatePreview donne un retour immédiat pendant la saisie d'une date.
type DatePreview struct {
	Display    string `json:"display"`
	Precision  string `json:"precision"`
	Calendar   string `json:"calendar"`
	Structured bool   `json:"structured"`
}

// PreviewDate interprète une saisie de date sans rien enregistrer.
func (a *API) PreviewDate(text string) DatePreview {
	d := domain.ParseDate(text)
	return DatePreview{Display: d.Format(domain.StyleLong), Precision: string(d.Precision),
		Calendar: string(d.Calendar), Structured: d.Structured()}
}

// store renvoie le dépôt du projet ouvert.
func (a *API) store() (*repo.Store, error) {
	p, err := a.projects.Require()
	if err != nil {
		return nil, err
	}
	return p.Store, nil
}

// write exécute une écriture puis notifie le frontend.
func (a *API) write(fn func(s *repo.Store) error) error {
	s, err := a.store()
	if err != nil {
		return err
	}
	if err := fn(s); err != nil {
		return err
	}
	a.emit(EventHistoryChanged, nil)
	return nil
}

// --- Lecture ---------------------------------------------------------------

func (a *API) GetPerson(id string) (PersonDetail, error) {
	s, err := a.store()
	if err != nil {
		return PersonDetail{}, err
	}
	return s.GetPersonDetail(a.context(), id)
}

func (a *API) SearchPersons(query string) ([]PersonSummary, error) {
	s, err := a.store()
	if err != nil {
		return nil, err
	}
	return s.SearchPersons(a.context(), query, 50)
}

func (a *API) SosaList() ([]SosaEntry, error) {
	s, err := a.store()
	if err != nil {
		return nil, err
	}
	return s.SosaList(a.context())
}

// GetRoot renvoie l'identifiant de la personne racine ("" si aucune).
func (a *API) GetRoot() (string, error) {
	s, err := a.store()
	if err != nil {
		return "", err
	}
	return s.Root(a.context())
}

func (a *API) SetRoot(personID string) error {
	return a.write(func(s *repo.Store) error { return s.SetRoot(a.context(), personID) })
}

// --- Personnes et noms -----------------------------------------------------

func (a *API) UpdatePerson(id string, p PersonPatch) error {
	return a.write(func(s *repo.Store) error { return s.UpdatePerson(a.context(), id, p) })
}

func (a *API) AddName(personID string, n NameInput) (string, error) {
	var id string
	err := a.write(func(s *repo.Store) (err error) { id, err = s.AddName(a.context(), personID, n); return })
	return id, err
}

func (a *API) UpdateName(nameID string, n NameInput) error {
	return a.write(func(s *repo.Store) error { return s.UpdateName(a.context(), nameID, n) })
}

func (a *API) SetPrimaryName(nameID string) error {
	return a.write(func(s *repo.Store) error { return s.SetPrimaryName(a.context(), nameID) })
}

func (a *API) DeleteName(nameID string) error {
	return a.write(func(s *repo.Store) error { return s.DeleteName(a.context(), nameID) })
}

// CreateRelative ajoute un proche : relation = father | mother | spouse | child | sibling.
func (a *API) CreateRelative(personID, relation string, in PersonInput, unionID string) (string, error) {
	var id string
	err := a.write(func(s *repo.Store) (err error) {
		id, err = s.CreateRelative(a.context(), personID, repo.Relation(relation),
			repo.NewPerson{Surname: in.Surname, GivenNames: in.GivenNames, Sex: in.Sex},
			repo.RelativeOpts{UnionID: unionID})
		return
	})
	return id, err
}

// DeleteImpact donne l'aperçu d'une suppression : mode = only | descendants | ancestors.
func (a *API) DeleteImpact(id, mode string) (Impact, error) {
	s, err := a.store()
	if err != nil {
		return Impact{}, err
	}
	return s.DeleteImpact(a.context(), id, repo.DeleteMode(mode))
}

func (a *API) DeletePeople(id, mode string) error {
	return a.write(func(s *repo.Store) error { return s.DeletePeople(a.context(), id, repo.DeleteMode(mode)) })
}

// --- Filiation et unions ---------------------------------------------------

func (a *API) LinkParent(childID, parentID, role, kind string) (string, error) {
	var id string
	err := a.write(func(s *repo.Store) (err error) {
		id, err = s.LinkParent(a.context(), childID, parentID, role, kind)
		return
	})
	return id, err
}

func (a *API) UnlinkParent(linkID string) error {
	return a.write(func(s *repo.Store) error { return s.UnlinkParent(a.context(), linkID) })
}

func (a *API) UpdateParentLink(linkID string, p ParentLinkPatch) error {
	return a.write(func(s *repo.Store) error { return s.UpdateParentLink(a.context(), linkID, p) })
}

func (a *API) ReorderChildren(linkIDs []string) error {
	return a.write(func(s *repo.Store) error { return s.ReorderChildren(a.context(), linkIDs) })
}

func (a *API) CreateUnion(typ string, partnerIDs []string) (string, error) {
	var id string
	err := a.write(func(s *repo.Store) (err error) { id, err = s.CreateUnion(a.context(), typ, partnerIDs); return })
	return id, err
}

func (a *API) UpdateUnion(id string, p UnionPatch) error {
	return a.write(func(s *repo.Store) error { return s.UpdateUnion(a.context(), id, p) })
}

func (a *API) DeleteUnion(id string) error {
	return a.write(func(s *repo.Store) error { return s.DeleteUnion(a.context(), id) })
}

// --- Événements et lieux ---------------------------------------------------

func (a *API) AddEvent(in EventInput) (string, error) {
	var id string
	err := a.write(func(s *repo.Store) (err error) { id, err = s.AddEvent(a.context(), in); return })
	return id, err
}

func (a *API) UpdateEvent(id string, p EventPatch) error {
	return a.write(func(s *repo.Store) error { return s.UpdateEvent(a.context(), id, p) })
}

func (a *API) DeleteEvent(id string) error {
	return a.write(func(s *repo.Store) error { return s.DeleteEvent(a.context(), id) })
}

func (a *API) ListPlaces(query string) ([]PlaceDTO, error) {
	s, err := a.store()
	if err != nil {
		return nil, err
	}
	return s.ListPlaces(a.context(), query, 200)
}

func (a *API) CreatePlace(in PlaceInput) (string, error) {
	var id string
	err := a.write(func(s *repo.Store) (err error) { id, err = s.CreatePlace(a.context(), in); return })
	return id, err
}

func (a *API) UpdatePlace(id string, in PlaceInput) error {
	return a.write(func(s *repo.Store) error { return s.UpdatePlace(a.context(), id, in) })
}

func (a *API) DeletePlace(id string) error {
	return a.write(func(s *repo.Store) error { return s.DeletePlace(a.context(), id) })
}
