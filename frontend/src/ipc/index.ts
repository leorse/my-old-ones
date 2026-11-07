// Client IPC typé : seul point d'accès du frontend au backend Go (spec §2).
// Les types viennent de wailsjs/go/models.ts, générés depuis les DTO Go
// (`wails generate module`, lancé automatiquement par `wails dev|build`).
import * as API from '../../wailsjs/go/api/API'
import {EventsOn} from '../../wailsjs/runtime/runtime'

export type {api, repo, jobs, providers, gedcom} from '../../wailsjs/go/models'

export const ipc = {
    appInfo: API.AppInfo,
    recentProjects: API.RecentProjects,
    currentProject: API.CurrentProject,
    newProjectDialog: API.NewProjectDialog,
    openProjectDialog: API.OpenProjectDialog,
    openProject: API.OpenProject,
    createProject: API.CreateProject,
    closeProject: API.CloseProject,
    listPersons: API.ListPersons,
    createPerson: API.CreatePerson,
    deletePerson: API.DeletePerson,
    historyState: API.HistoryState,
    undo: API.Undo,
    redo: API.Redo,
    listJobs: API.ListJobs,
    cancelJob: API.CancelJob,
    getPerson: API.GetPerson,
    getTree: API.GetTree,
    gedcomDialog: API.GedcomDialog,
    importGedcomNewProject: API.ImportGedcomNewProject,
    importGedcomHere: API.ImportGedcomHere,
    gedcomReport: API.GedcomReport,
    sampleDialog: API.SampleDialog,
    createSampleProject: API.CreateSampleProject,
    updatePerson: API.UpdatePerson,
    addName: API.AddName,
    updateName: API.UpdateName,
    setPrimaryName: API.SetPrimaryName,
    deleteName: API.DeleteName,
    createRelative: API.CreateRelative,
    deleteImpact: API.DeleteImpact,
    deletePeople: API.DeletePeople,
    linkParent: API.LinkParent,
    unlinkParent: API.UnlinkParent,
    updateParentLink: API.UpdateParentLink,
    reorderChildren: API.ReorderChildren,
    createUnion: API.CreateUnion,
    updateUnion: API.UpdateUnion,
    deleteUnion: API.DeleteUnion,
    addEvent: API.AddEvent,
    updateEvent: API.UpdateEvent,
    deleteEvent: API.DeleteEvent,
    previewDate: API.PreviewDate,
    listPlaces: API.ListPlaces,
    createPlace: API.CreatePlace,
    updatePlace: API.UpdatePlace,
    deletePlace: API.DeletePlace,
    searchPersons: API.SearchPersons,
    sosaList: API.SosaList,
    getRoot: API.GetRoot,
    setRoot: API.SetRoot,
    listProviders: API.ListProviders,
    setProviderEnabled: API.SetProviderEnabled,
}

/** Noms des événements émis par le backend (internal/api, internal/jobs). */
export const Events = {
    projectChanged: 'project:changed',
    historyChanged: 'history:changed',
    jobUpdate: 'job:update',
} as const

/** S'abonne à un événement backend ; renvoie la fonction de désabonnement. */
export function onEvent<T = unknown>(name: string, cb: (data: T) => void): () => void {
    return EventsOn(name, cb)
}
