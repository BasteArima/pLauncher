export namespace main {
	
	export class LauncherUpdate {
	    available: boolean;
	    can_install: boolean;
	    current: string;
	    latest: string;
	    notes: string;
	    page_url: string;
	    asset_url: string;
	    asset_name: string;
	    size: number;
	    managed: string;
	
	    static createFrom(source: any = {}) {
	        return new LauncherUpdate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.can_install = source["can_install"];
	        this.current = source["current"];
	        this.latest = source["latest"];
	        this.notes = source["notes"];
	        this.page_url = source["page_url"];
	        this.asset_url = source["asset_url"];
	        this.asset_name = source["asset_name"];
	        this.size = source["size"];
	        this.managed = source["managed"];
	    }
	}
	export class MissingGame {
	    id: string;
	    title: string;
	    cover_path: string;
	    old_path: string;
	    candidate: string;
	    duplicate_id: string;
	    duplicate_of: string;
	
	    static createFrom(source: any = {}) {
	        return new MissingGame(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.cover_path = source["cover_path"];
	        this.old_path = source["old_path"];
	        this.candidate = source["candidate"];
	        this.duplicate_id = source["duplicate_id"];
	        this.duplicate_of = source["duplicate_of"];
	    }
	}
	export class PrivacySettings {
	    has_pin: boolean;
	    locked: boolean;
	    idle_lock_min: number;
	    panic_enabled: boolean;
	    panic_mods: number;
	    panic_vk: number;
	    panic_label: string;
	    panic_action: string;
	    lock_on_panic: boolean;
	    blur_mode: string;
	    start_discreet: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PrivacySettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.has_pin = source["has_pin"];
	        this.locked = source["locked"];
	        this.idle_lock_min = source["idle_lock_min"];
	        this.panic_enabled = source["panic_enabled"];
	        this.panic_mods = source["panic_mods"];
	        this.panic_vk = source["panic_vk"];
	        this.panic_label = source["panic_label"];
	        this.panic_action = source["panic_action"];
	        this.lock_on_panic = source["lock_on_panic"];
	        this.blur_mode = source["blur_mode"];
	        this.start_discreet = source["start_discreet"];
	    }
	}

}

export namespace models {
	
	export class Collection {
	    id: string;
	    name: string;
	    type: string;
	    game_ids: string[];
	    tags: string[];
	    hidden: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Collection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.game_ids = source["game_ids"];
	        this.tags = source["tags"];
	        this.hidden = source["hidden"];
	    }
	}
	export class GameSource {
	    source: string;
	    url: string;
	    last_version: string;
	
	    static createFrom(source: any = {}) {
	        return new GameSource(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.url = source["url"];
	        this.last_version = source["last_version"];
	    }
	}
	export class Game {
	    id: string;
	    title: string;
	    description: string;
	    version: string;
	    author: string;
	    engine: string;
	    languages: string[];
	    cover_path: string;
	    cover_fit: string;
	    cover_pos: string;
	    images: string[];
	    tags: string[];
	    exec_path: string;
	    folder_path: string;
	    favorite: boolean;
	    time_played: number;
	    added_at: number;
	    last_launched_at: number;
	    sources: GameSource[];
	    primary_source: string;
	    update_available: boolean;
	    update_version: string;
	    update_source: string;
	    last_checked_at: number;
	    size_bytes: number;
	    size_checked_at: number;
	    folder_missing: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Game(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.version = source["version"];
	        this.author = source["author"];
	        this.engine = source["engine"];
	        this.languages = source["languages"];
	        this.cover_path = source["cover_path"];
	        this.cover_fit = source["cover_fit"];
	        this.cover_pos = source["cover_pos"];
	        this.images = source["images"];
	        this.tags = source["tags"];
	        this.exec_path = source["exec_path"];
	        this.folder_path = source["folder_path"];
	        this.favorite = source["favorite"];
	        this.time_played = source["time_played"];
	        this.added_at = source["added_at"];
	        this.last_launched_at = source["last_launched_at"];
	        this.sources = this.convertValues(source["sources"], GameSource);
	        this.primary_source = source["primary_source"];
	        this.update_available = source["update_available"];
	        this.update_version = source["update_version"];
	        this.update_source = source["update_source"];
	        this.last_checked_at = source["last_checked_at"];
	        this.size_bytes = source["size_bytes"];
	        this.size_checked_at = source["size_checked_at"];
	        this.folder_missing = source["folder_missing"];
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

export namespace parser {
	
	export class Source {
	    name: string;
	    domain: string;
	
	    static createFrom(source: any = {}) {
	        return new Source(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.domain = source["domain"];
	    }
	}

}

