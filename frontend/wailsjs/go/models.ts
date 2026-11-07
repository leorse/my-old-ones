export namespace api {
	
	export class AppInfo {
	    version: string;
	    schemaVersion: number;
	
	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.schemaVersion = source["schemaVersion"];
	    }
	}
	export class DatePreview {
	    display: string;
	    precision: string;
	    calendar: string;
	    structured: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DatePreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.display = source["display"];
	        this.precision = source["precision"];
	        this.calendar = source["calendar"];
	        this.structured = source["structured"];
	    }
	}
	export class PersonInput {
	    surname: string;
	    givenNames: string[];
	    sex: string;
	
	    static createFrom(source: any = {}) {
	        return new PersonInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.surname = source["surname"];
	        this.givenNames = source["givenNames"];
	        this.sex = source["sex"];
	    }
	}
	export class ProjectInfo {
	    path: string;
	    name: string;
	    mediaDir: string;
	    persons: number;
	
	    static createFrom(source: any = {}) {
	        return new ProjectInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.mediaDir = source["mediaDir"];
	        this.persons = source["persons"];
	    }
	}
	export class RecentProject {
	    path: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new RecentProject(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	    }
	}

}

export namespace domain {
	
	export class Date {
	    precision: string;
	    calendar: string;
	    y1: number;
	    m1: number;
	    d1: number;
	    y2: number;
	    m2: number;
	    d2: number;
	    originalText: string;
	
	    static createFrom(source: any = {}) {
	        return new Date(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.precision = source["precision"];
	        this.calendar = source["calendar"];
	        this.y1 = source["y1"];
	        this.m1 = source["m1"];
	        this.d1 = source["d1"];
	        this.y2 = source["y2"];
	        this.m2 = source["m2"];
	        this.d2 = source["d2"];
	        this.originalText = source["originalText"];
	    }
	}

}

export namespace gedcom {
	
	export class Report {
	    fileName: string;
	    software: string;
	    gedcomVersion: string;
	    encoding: string;
	    declaredEncoding: string;
	    individuals: number;
	    families: number;
	    unions: number;
	    parentLinks: number;
	    names: number;
	    events: number;
	    places: number;
	    sources: number;
	    citations: number;
	    notes: number;
	    extraTags: number;
	    mediaSkipped: number;
	    autoDeceased: number;
	    skipped: Record<string, number>;
	    warnings: string[];
	    warningsDropped: number;
	    corrections: string[];
	    durationMs: number;
	
	    static createFrom(source: any = {}) {
	        return new Report(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fileName = source["fileName"];
	        this.software = source["software"];
	        this.gedcomVersion = source["gedcomVersion"];
	        this.encoding = source["encoding"];
	        this.declaredEncoding = source["declaredEncoding"];
	        this.individuals = source["individuals"];
	        this.families = source["families"];
	        this.unions = source["unions"];
	        this.parentLinks = source["parentLinks"];
	        this.names = source["names"];
	        this.events = source["events"];
	        this.places = source["places"];
	        this.sources = source["sources"];
	        this.citations = source["citations"];
	        this.notes = source["notes"];
	        this.extraTags = source["extraTags"];
	        this.mediaSkipped = source["mediaSkipped"];
	        this.autoDeceased = source["autoDeceased"];
	        this.skipped = source["skipped"];
	        this.warnings = source["warnings"];
	        this.warningsDropped = source["warningsDropped"];
	        this.corrections = source["corrections"];
	        this.durationMs = source["durationMs"];
	    }
	}

}

export namespace jobs {
	
	export class Status {
	    id: string;
	    kind: string;
	    state: string;
	    done: number;
	    total: number;
	    message: string;
	    error?: string;
	    // Go type: time
	    startedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.state = source["state"];
	        this.done = source["done"];
	        this.total = source["total"];
	        this.message = source["message"];
	        this.error = source["error"];
	        this.startedAt = this.convertValues(source["startedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace providers {
	
	export class Info {
	    id: string;
	    name: string;
	    description: string;
	    termsUrl: string;
	    kinds: string[];
	    ratePerSec: number;
	    needsKey: boolean;
	    priority: number;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.termsUrl = source["termsUrl"];
	        this.kinds = source["kinds"];
	        this.ratePerSec = source["ratePerSec"];
	        this.needsKey = source["needsKey"];
	        this.priority = source["priority"];
	    }
	}
	export class Entry {
	    info: Info;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.info = this.convertValues(source["info"], Info);
	        this.enabled = source["enabled"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace repo {
	
	export class PersonSummary {
	    id: string;
	    surname: string;
	    givenNames: string[];
	    sex: string;
	    living: string;
	
	    static createFrom(source: any = {}) {
	        return new PersonSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.surname = source["surname"];
	        this.givenNames = source["givenNames"];
	        this.sex = source["sex"];
	        this.living = source["living"];
	    }
	}
	export class ChildRef {
	    linkId: string;
	    kind: string;
	    person: PersonSummary;
	
	    static createFrom(source: any = {}) {
	        return new ChildRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.linkId = source["linkId"];
	        this.kind = source["kind"];
	        this.person = this.convertValues(source["person"], PersonSummary);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class EventAge {
	    eventId: string;
	    years: number;
	    approx: boolean;
	
	    static createFrom(source: any = {}) {
	        return new EventAge(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.eventId = source["eventId"];
	        this.years = source["years"];
	        this.approx = source["approx"];
	    }
	}
	export class EventDTO {
	    id: string;
	    type: string;
	    personId: string;
	    unionId: string;
	    date: domain.Date;
	    dateDisplay: string;
	    placeId: string;
	    placeName: string;
	    description: string;
	    notesMd: string;
	
	    static createFrom(source: any = {}) {
	        return new EventDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.personId = source["personId"];
	        this.unionId = source["unionId"];
	        this.date = this.convertValues(source["date"], domain.Date);
	        this.dateDisplay = source["dateDisplay"];
	        this.placeId = source["placeId"];
	        this.placeName = source["placeName"];
	        this.description = source["description"];
	        this.notesMd = source["notesMd"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class EventInput {
	    type: string;
	    personId: string;
	    unionId: string;
	    date: string;
	    placeId: string;
	    description: string;
	    notesMd: string;
	
	    static createFrom(source: any = {}) {
	        return new EventInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.personId = source["personId"];
	        this.unionId = source["unionId"];
	        this.date = source["date"];
	        this.placeId = source["placeId"];
	        this.description = source["description"];
	        this.notesMd = source["notesMd"];
	    }
	}
	export class EventPatch {
	    type?: string;
	    date?: string;
	    placeId?: string;
	    description?: string;
	    notesMd?: string;
	
	    static createFrom(source: any = {}) {
	        return new EventPatch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.date = source["date"];
	        this.placeId = source["placeId"];
	        this.description = source["description"];
	        this.notesMd = source["notesMd"];
	    }
	}
	export class UnionView {
	    id: string;
	    type: string;
	    status: string;
	    partners: PersonSummary[];
	    events: EventDTO[];
	    children: ChildRef[];
	
	    static createFrom(source: any = {}) {
	        return new UnionView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.status = source["status"];
	        this.partners = this.convertValues(source["partners"], PersonSummary);
	        this.events = this.convertValues(source["events"], EventDTO);
	        this.children = this.convertValues(source["children"], ChildRef);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ParentRef {
	    linkId: string;
	    role: string;
	    kind: string;
	    person: PersonSummary;
	
	    static createFrom(source: any = {}) {
	        return new ParentRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.linkId = source["linkId"];
	        this.role = source["role"];
	        this.kind = source["kind"];
	        this.person = this.convertValues(source["person"], PersonSummary);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Family {
	    parents: ParentRef[];
	    unions: UnionView[];
	    siblings: PersonSummary[];
	    otherChildren: ChildRef[];
	
	    static createFrom(source: any = {}) {
	        return new Family(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.parents = this.convertValues(source["parents"], ParentRef);
	        this.unions = this.convertValues(source["unions"], UnionView);
	        this.siblings = this.convertValues(source["siblings"], PersonSummary);
	        this.otherChildren = this.convertValues(source["otherChildren"], ChildRef);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class HistoryState {
	    canUndo: boolean;
	    canRedo: boolean;
	    undoLabel: string;
	    redoLabel: string;
	
	    static createFrom(source: any = {}) {
	        return new HistoryState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.canUndo = source["canUndo"];
	        this.canRedo = source["canRedo"];
	        this.undoLabel = source["undoLabel"];
	        this.redoLabel = source["redoLabel"];
	    }
	}
	export class Impact {
	    people: PersonSummary[];
	    events: number;
	    parentLinks: number;
	    unions: number;
	    mediaLinks: number;
	    citations: number;
	    becomingIsolated: PersonSummary[];
	
	    static createFrom(source: any = {}) {
	        return new Impact(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.people = this.convertValues(source["people"], PersonSummary);
	        this.events = source["events"];
	        this.parentLinks = source["parentLinks"];
	        this.unions = source["unions"];
	        this.mediaLinks = source["mediaLinks"];
	        this.citations = source["citations"];
	        this.becomingIsolated = this.convertValues(source["becomingIsolated"], PersonSummary);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class NameDTO {
	    id: string;
	    type: string;
	    surname: string;
	    givenNames: string[];
	    usualGiven: number;
	    prefix: string;
	    suffix: string;
	    isPrimary: boolean;
	
	    static createFrom(source: any = {}) {
	        return new NameDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.surname = source["surname"];
	        this.givenNames = source["givenNames"];
	        this.usualGiven = source["usualGiven"];
	        this.prefix = source["prefix"];
	        this.suffix = source["suffix"];
	        this.isPrimary = source["isPrimary"];
	    }
	}
	export class NameInput {
	    type: string;
	    surname: string;
	    givenNames: string[];
	    usualGiven: number;
	    prefix: string;
	    suffix: string;
	
	    static createFrom(source: any = {}) {
	        return new NameInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.surname = source["surname"];
	        this.givenNames = source["givenNames"];
	        this.usualGiven = source["usualGiven"];
	        this.prefix = source["prefix"];
	        this.suffix = source["suffix"];
	    }
	}
	export class ParentLinkPatch {
	    role?: string;
	    kind?: string;
	    unionId?: string;
	
	    static createFrom(source: any = {}) {
	        return new ParentLinkPatch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.kind = source["kind"];
	        this.unionId = source["unionId"];
	    }
	}
	
	export class PersonCore {
	    id: string;
	    sex: string;
	    living: string;
	    isFavorite: boolean;
	    notesMd: string;
	    createdAt: string;
	    updatedAt: string;
	    names: NameDTO[];
	
	    static createFrom(source: any = {}) {
	        return new PersonCore(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sex = source["sex"];
	        this.living = source["living"];
	        this.isFavorite = source["isFavorite"];
	        this.notesMd = source["notesMd"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.names = this.convertValues(source["names"], NameDTO);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PersonDetail {
	    core: PersonCore;
	    events: EventDTO[];
	    family: Family;
	    ages: EventAge[];
	    ageAtDeath?: EventAge;
	    sosa: number[];
	    isRoot: boolean;
	    parentAges: EventAge[];
	
	    static createFrom(source: any = {}) {
	        return new PersonDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.core = this.convertValues(source["core"], PersonCore);
	        this.events = this.convertValues(source["events"], EventDTO);
	        this.family = this.convertValues(source["family"], Family);
	        this.ages = this.convertValues(source["ages"], EventAge);
	        this.ageAtDeath = this.convertValues(source["ageAtDeath"], EventAge);
	        this.sosa = source["sosa"];
	        this.isRoot = source["isRoot"];
	        this.parentAges = this.convertValues(source["parentAges"], EventAge);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PersonPatch {
	    sex?: string;
	    living?: string;
	    isFavorite?: boolean;
	    notesMd?: string;
	
	    static createFrom(source: any = {}) {
	        return new PersonPatch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sex = source["sex"];
	        this.living = source["living"];
	        this.isFavorite = source["isFavorite"];
	        this.notesMd = source["notesMd"];
	    }
	}
	
	export class PlaceDTO {
	    id: string;
	    name: string;
	    type: string;
	    parentId: string;
	    latitude?: number;
	    longitude?: number;
	    codeInsee: string;
	    validFrom: string;
	    validTo: string;
	    displayName: string;
	
	    static createFrom(source: any = {}) {
	        return new PlaceDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.parentId = source["parentId"];
	        this.latitude = source["latitude"];
	        this.longitude = source["longitude"];
	        this.codeInsee = source["codeInsee"];
	        this.validFrom = source["validFrom"];
	        this.validTo = source["validTo"];
	        this.displayName = source["displayName"];
	    }
	}
	export class PlaceInput {
	    name: string;
	    type: string;
	    parentId: string;
	    latitude?: number;
	    longitude?: number;
	    codeInsee: string;
	    validFrom: string;
	    validTo: string;
	
	    static createFrom(source: any = {}) {
	        return new PlaceInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.parentId = source["parentId"];
	        this.latitude = source["latitude"];
	        this.longitude = source["longitude"];
	        this.codeInsee = source["codeInsee"];
	        this.validFrom = source["validFrom"];
	        this.validTo = source["validTo"];
	    }
	}
	export class SosaEntry {
	    number: number;
	    person: PersonSummary;
	
	    static createFrom(source: any = {}) {
	        return new SosaEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.number = source["number"];
	        this.person = this.convertValues(source["person"], PersonSummary);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TreeUnion {
	    id: string;
	    type: string;
	    status: string;
	    partners: string[];
	    year: string;
	
	    static createFrom(source: any = {}) {
	        return new TreeUnion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.status = source["status"];
	        this.partners = source["partners"];
	        this.year = source["year"];
	    }
	}
	export class TreeLink {
	    id: string;
	    childId: string;
	    parentId: string;
	    role: string;
	    kind: string;
	    unionId: string;
	
	    static createFrom(source: any = {}) {
	        return new TreeLink(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.childId = source["childId"];
	        this.parentId = source["parentId"];
	        this.role = source["role"];
	        this.kind = source["kind"];
	        this.unionId = source["unionId"];
	    }
	}
	export class TreeNode {
	    person: PersonSummary;
	    birth: string;
	    death: string;
	    hasFather: boolean;
	    hasMother: boolean;
	    hasParents: boolean;
	    childCount: number;
	
	    static createFrom(source: any = {}) {
	        return new TreeNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.person = this.convertValues(source["person"], PersonSummary);
	        this.birth = source["birth"];
	        this.death = source["death"];
	        this.hasFather = source["hasFather"];
	        this.hasMother = source["hasMother"];
	        this.hasParents = source["hasParents"];
	        this.childCount = source["childCount"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TreeData {
	    focus: string;
	    nodes: TreeNode[];
	    links: TreeLink[];
	    unions: TreeUnion[];
	    truncated: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TreeData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.focus = source["focus"];
	        this.nodes = this.convertValues(source["nodes"], TreeNode);
	        this.links = this.convertValues(source["links"], TreeLink);
	        this.unions = this.convertValues(source["unions"], TreeUnion);
	        this.truncated = source["truncated"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	export class UnionPatch {
	    type?: string;
	    status?: string;
	    notesMd?: string;
	
	    static createFrom(source: any = {}) {
	        return new UnionPatch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.status = source["status"];
	        this.notesMd = source["notesMd"];
	    }
	}

}

