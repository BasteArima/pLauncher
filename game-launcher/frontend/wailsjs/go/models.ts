export namespace models {
	
	export class Game {
	    id: string;
	    title: string;
	    description: string;
	    version: string;
	    languages: string[];
	    cover_path: string;
	    images: string[];
	    exec_path: string;
	    folder_path: string;
	    time_played: number;
	
	    static createFrom(source: any = {}) {
	        return new Game(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.version = source["version"];
	        this.languages = source["languages"];
	        this.cover_path = source["cover_path"];
	        this.images = source["images"];
	        this.exec_path = source["exec_path"];
	        this.folder_path = source["folder_path"];
	        this.time_played = source["time_played"];
	    }
	}

}

