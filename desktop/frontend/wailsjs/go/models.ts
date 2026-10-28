export namespace audit {
	
	export class Event {
	    // Go type: time
	    time: any;
	    user: string;
	    action: string;
	    target?: string;
	    success: boolean;
	    detail?: string;
	
	    static createFrom(source: any = {}) {
	        return new Event(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = this.convertValues(source["time"], null);
	        this.user = source["user"];
	        this.action = source["action"];
	        this.target = source["target"];
	        this.success = source["success"];
	        this.detail = source["detail"];
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

export namespace config {
	
	export class Target {
	    name: string;
	    kind: string;
	    host: string;
	    user?: string;
	    port?: number;
	    private_key?: string;
	    password_enc?: string;
	    package_manager?: string;
	    tags?: string[];
	    enabled?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Target(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.host = source["host"];
	        this.user = source["user"];
	        this.port = source["port"];
	        this.private_key = source["private_key"];
	        this.password_enc = source["password_enc"];
	        this.package_manager = source["package_manager"];
	        this.tags = source["tags"];
	        this.enabled = source["enabled"];
	    }
	}

}

export namespace executor {
	
	export class HostResult {
	    name: string;
	    kind: string;
	    host: string;
	    os?: string;
	    updates?: patch.Update[];
	    reboot_required: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new HostResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.host = source["host"];
	        this.os = source["os"];
	        this.updates = this.convertValues(source["updates"], patch.Update);
	        this.reboot_required = source["reboot_required"];
	        this.error = source["error"];
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

export namespace inventory {
	
	export class Tech {
	    name: string;
	    version: string;
	    source: string;
	    path?: string;
	
	    static createFrom(source: any = {}) {
	        return new Tech(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.source = source["source"];
	        this.path = source["path"];
	    }
	}
	export class HostInventory {
	    name: string;
	    kind: string;
	    host: string;
	    os?: string;
	    techs: Tech[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new HostInventory(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.host = source["host"];
	        this.os = source["os"];
	        this.techs = this.convertValues(source["techs"], Tech);
	        this.error = source["error"];
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

export namespace patch {
	
	export class Update {
	    package: string;
	    current_version: string;
	    available_version: string;
	    severity: string;
	
	    static createFrom(source: any = {}) {
	        return new Update(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.package = source["package"];
	        this.current_version = source["current_version"];
	        this.available_version = source["available_version"];
	        this.severity = source["severity"];
	    }
	}

}

