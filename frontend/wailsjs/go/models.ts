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
	export class HistoryEntry {
	    name: string;
	    path: string;
	    size: number;
	    modTime: string;
	
	    static createFrom(source: any = {}) {
	        return new HistoryEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.size = source["size"];
	        this.modTime = source["modTime"];
	    }
	}
	export class Metrics {
	    bytesIn: number;
	    bytesOut: number;
	    keepAliveMs: number;
	
	    static createFrom(source: any = {}) {
	        return new Metrics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bytesIn = source["bytesIn"];
	        this.bytesOut = source["bytesOut"];
	        this.keepAliveMs = source["keepAliveMs"];
	    }
	}
	export class SshConfig {
	    protocol: string;
	    host: string;
	    port: number;
	    username: string;
	    password: string;
	    privateKey: string;
	    privateKeyPath: string;
	    passphrase: string;
	    otp: string;
	    jumpHost: string;
	    jumpPort: number;
	    jumpUser: string;
	    jumpPassword: string;
	    jumpPrivateKeyPath: string;
	    jumpPassphrase: string;
	
	    static createFrom(source: any = {}) {
	        return new SshConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.protocol = source["protocol"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.privateKey = source["privateKey"];
	        this.privateKeyPath = source["privateKeyPath"];
	        this.passphrase = source["passphrase"];
	        this.otp = source["otp"];
	        this.jumpHost = source["jumpHost"];
	        this.jumpPort = source["jumpPort"];
	        this.jumpUser = source["jumpUser"];
	        this.jumpPassword = source["jumpPassword"];
	        this.jumpPrivateKeyPath = source["jumpPrivateKeyPath"];
	        this.jumpPassphrase = source["jumpPassphrase"];
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
	export class SysMetrics {
	    cpuPercent: number;
	    memUsed: number;
	    memTotal: number;
	    netIn: number;
	    netOut: number;
	    uptime: number;
	
	    static createFrom(source: any = {}) {
	        return new SysMetrics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cpuPercent = source["cpuPercent"];
	        this.memUsed = source["memUsed"];
	        this.memTotal = source["memTotal"];
	        this.netIn = source["netIn"];
	        this.netOut = source["netOut"];
	        this.uptime = source["uptime"];
	    }
	}
	export class TransferTask {
	    taskId: number;
	    sessionId: number;
	    direction: string;
	    localPath: string;
	    remotePath: string;
	    currentFile: string;
	    size: number;
	    transferred: number;
	    status: string;
	    error: string;
	    conflict: string;
	    isDir: boolean;
	
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
	        this.currentFile = source["currentFile"];
	        this.size = source["size"];
	        this.transferred = source["transferred"];
	        this.status = source["status"];
	        this.error = source["error"];
	        this.conflict = source["conflict"];
	        this.isDir = source["isDir"];
	    }
	}
	export class Tunnel {
	    id: number;
	    sessionId: number;
	    type: string;
	    listenAddr: string;
	    targetAddr: string;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new Tunnel(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sessionId = source["sessionId"];
	        this.type = source["type"];
	        this.listenAddr = source["listenAddr"];
	        this.targetAddr = source["targetAddr"];
	        this.status = source["status"];
	    }
	}

}

