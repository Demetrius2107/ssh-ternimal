export namespace model {
	
	export class FileEntry {
	    name: string;
	    path: string;
	    isDir: boolean;
	    size: number;
	    modTime: string;
	    mode: string;
	
	    static createFrom(source: any = {}) {
	        return new FileEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.isDir = source["isDir"];
	        this.size = source["size"];
	        this.modTime = source["modTime"];
	        this.mode = source["mode"];
	    }
	}
	export class SshConfig {
	    host: string;
	    port: number;
	    username: string;
	    password: string;
	    privateKey: string;
	    passphrase: string;
	
	    static createFrom(source: any = {}) {
	        return new SshConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.privateKey = source["privateKey"];
	        this.passphrase = source["passphrase"];
	    }
	}
	export class StoredSession {
	    id: string;
	    name: string;
	    host: string;
	    port: number;
	    username: string;
	
	    static createFrom(source: any = {}) {
	        return new StoredSession(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.username = source["username"];
	    }
	}
	export class TransferTask {
	    taskId: number;
	    sessionId: number;
	    direction: string;
	    localPath: string;
	    remotePath: string;
	    size: number;
	    transferred: number;
	    status: string;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new TransferTask(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.taskId = source["taskId"];
	        this.sessionId = source["sessionId"];
	        this.direction = source["direction"];
	        this.localPath = source["localPath"];
	        this.remotePath = source["remotePath"];
	        this.size = source["size"];
	        this.transferred = source["transferred"];
	        this.status = source["status"];
	        this.error = source["error"];
	    }
	}

}

