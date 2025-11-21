package server

import "gpt-clickup/src/model"

// MemoryClickUpRepository offre un archivio in-memory utile per i test.
type MemoryClickUpRepository struct {
	workspaces []model.WorkspaceClickUp
	spaces     map[string][]model.SpaceClickUp
	lists      map[string][]model.ListClickUp
	folders    map[string][]model.FolderClickUp
	tasks      map[string][]model.TaskClickUp
}

// NewMemoryClickUpRepository crea un repository volatile che può essere azzerato facilmente.
func NewMemoryClickUpRepository() *MemoryClickUpRepository {
	return &MemoryClickUpRepository{
		spaces:  make(map[string][]model.SpaceClickUp),
		lists:   make(map[string][]model.ListClickUp),
		folders: make(map[string][]model.FolderClickUp),
		tasks:   make(map[string][]model.TaskClickUp),
	}
}

// Reset permette di cancellare tutti i dati caricati durante un test.
func (m *MemoryClickUpRepository) Reset() {
	m.workspaces = nil
	m.spaces = make(map[string][]model.SpaceClickUp)
	m.lists = make(map[string][]model.ListClickUp)
	m.folders = make(map[string][]model.FolderClickUp)
	m.tasks = make(map[string][]model.TaskClickUp)
}

func (m *MemoryClickUpRepository) GetWorkspaces() ([]model.WorkspaceClickUp, error) {
	return m.workspaces, nil
}

func (m *MemoryClickUpRepository) GetWorkspaceTree() ([]model.WorkspaceClickUp, error) {
	result := make([]model.WorkspaceClickUp, len(m.workspaces))
	for i, ws := range m.workspaces {
		wsCopy := ws
		spaces := m.spaces[ws.ID]
		wsCopy.Spaces = make([]model.SpaceClickUp, len(spaces))
		for j, space := range spaces {
			spaceCopy := space
			// collega le liste di primo livello
			lists := m.lists[space.ID]
			topLevel := make([]model.ListClickUp, 0)
			folderBuckets := make(map[string][]model.ListClickUp)
			for _, list := range lists {
				listCopy := list
				if tasks, ok := m.tasks[list.ID]; ok {
					listCopy.Tasks = append([]model.TaskClickUp(nil), tasks...)
				}
				if list.FolderID == nil {
					topLevel = append(topLevel, listCopy)
				} else {
					folderID := *list.FolderID
					folderBuckets[folderID] = append(folderBuckets[folderID], listCopy)
				}
			}
			spaceCopy.Lists = topLevel

			folders := m.folders[space.ID]
			spaceCopy.Folders = make([]model.FolderClickUp, len(folders))
			for k, folder := range folders {
				folderCopy := folder
				folderCopy.Lists = append([]model.ListClickUp(nil), folderBuckets[folder.ID]...)
				spaceCopy.Folders[k] = folderCopy
			}

			wsCopy.Spaces[j] = spaceCopy
		}
		result[i] = wsCopy
	}
	return result, nil
}

func (m *MemoryClickUpRepository) SaveWorkspaces(ws []model.WorkspaceClickUp) error {
	m.workspaces = ws
	return nil
}

func (m *MemoryClickUpRepository) GetSpaces(workspaceID string) ([]model.SpaceClickUp, error) {
	return m.spaces[workspaceID], nil
}

func (m *MemoryClickUpRepository) SaveSpaces(spaces []model.SpaceClickUp) error {
	for _, space := range spaces {
		m.spaces[space.WorkspaceID] = appendUniqueSpace(m.spaces[space.WorkspaceID], space)
	}
	return nil
}

func appendUniqueSpace(existing []model.SpaceClickUp, space model.SpaceClickUp) []model.SpaceClickUp {
	for i, s := range existing {
		if s.ID == space.ID {
			existing[i] = space
			return existing
		}
	}
	return append(existing, space)
}

func (m *MemoryClickUpRepository) GetLists(spaceID string) ([]model.ListClickUp, error) {
	lists := m.lists[spaceID]
	result := make([]model.ListClickUp, 0)
	for _, list := range lists {
		if list.FolderID == nil {
			result = append(result, list)
		}
	}
	return result, nil
}

func (m *MemoryClickUpRepository) SaveLists(lists []model.ListClickUp) error {
	for _, list := range lists {
		bucket := m.lists[list.SpaceID]
		updated := false
		for i, existing := range bucket {
			if existing.ID == list.ID {
				bucket[i] = list
				updated = true
				break
			}
		}
		if !updated {
			bucket = append(bucket, list)
		}
		m.lists[list.SpaceID] = bucket
	}
	return nil
}

func (m *MemoryClickUpRepository) GetFolders(spaceID string) ([]model.FolderClickUp, error) {
	return m.folders[spaceID], nil
}

func (m *MemoryClickUpRepository) SaveFolders(folders []model.FolderClickUp) error {
	for _, folder := range folders {
		bucket := m.folders[folder.SpaceID]
		replaced := false
		for i, existing := range bucket {
			if existing.ID == folder.ID {
				bucket[i] = folder
				replaced = true
				break
			}
		}
		if !replaced {
			bucket = append(bucket, folder)
		}
		m.folders[folder.SpaceID] = bucket
		if len(folder.Lists) > 0 {
			for idx := range folder.Lists {
				folder.Lists[idx].SpaceID = folder.SpaceID
				folderID := folder.ID
				folder.Lists[idx].FolderID = &folderID
			}
			_ = m.SaveLists(folder.Lists)
		}
	}
	return nil
}

func (m *MemoryClickUpRepository) GetTasks(listID string) ([]model.TaskClickUp, error) {
	return m.tasks[listID], nil
}

func (m *MemoryClickUpRepository) SaveTasks(tasks []model.TaskClickUp) error {
	for _, task := range tasks {
		bucket := m.tasks[task.ListID]
		replaced := false
		for i, existing := range bucket {
			if existing.ID == task.ID {
				bucket[i] = task
				replaced = true
				break
			}
		}
		if !replaced {
			bucket = append(bucket, task)
		}
		m.tasks[task.ListID] = bucket
	}
	return nil
}
