export namespace model {
	
	export class AiStatus {
	    provider: string;
	    model: string;
	    monthlyLimit: number;
	    monthUsage: number;
	    keyConfigured: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AiStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.model = source["model"];
	        this.monthlyLimit = source["monthlyLimit"];
	        this.monthUsage = source["monthUsage"];
	        this.keyConfigured = source["keyConfigured"];
	    }
	}
	export class AuditEntry {
	    id: string;
	    startTime: string;
	    endTime: string;
	    duration: number;
	    host: string;
	    port: number;
	    user: string;
	    protocol: string;
	    bytesIn: number;
	    bytesOut: number;
	    history: string;
	    commandLog: string;
	    label: string;
	
	    static createFrom(source: any = {}) {
	        return new AuditEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.startTime = source["startTime"];
	        this.endTime = source["endTime"];
	        this.duration = source["duration"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.user = source["user"];
	        this.protocol = source["protocol"];
	        this.bytesIn = source["bytesIn"];
	        this.bytesOut = source["bytesOut"];
	        this.history = source["history"];
	        this.commandLog = source["commandLog"];
	        this.label = source["label"];
	    }
	}
	export class CredentialListEntry {
	    id: string;
	    name: string;
	    type: string;
	    username: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new CredentialListEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.username = source["username"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class DiskUsage {
	    filesystem: string;
	    size: string;
	    used: string;
	    avail: string;
	    usePct: number;
	    mounted: string;
	
	    static createFrom(source: any = {}) {
	        return new DiskUsage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filesystem = source["filesystem"];
	        this.size = source["size"];
	        this.used = source["used"];
	        this.avail = source["avail"];
	        this.usePct = source["usePct"];
	        this.mounted = source["mounted"];
	    }
	}
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
	export class HistoryMatch {
	    name: string;
	    path: string;
	    count: number;
	    preview: string;
	
	    static createFrom(source: any = {}) {
	        return new HistoryMatch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.count = source["count"];
	        this.preview = source["preview"];
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
	export class PortInfo {
	    protocol: string;
	    addr: string;
	    port: string;
	    process: string;
	
	    static createFrom(source: any = {}) {
	        return new PortInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.protocol = source["protocol"];
	        this.addr = source["addr"];
	        this.port = source["port"];
	        this.process = source["process"];
	    }
	}
	export class ProcEntry {
	    pid: string;
	    user: string;
	    cpu: number;
	    mem: number;
	    command: string;
	
	    static createFrom(source: any = {}) {
	        return new ProcEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pid = source["pid"];
	        this.user = source["user"];
	        this.cpu = source["cpu"];
	        this.mem = source["mem"];
	        this.command = source["command"];
	    }
	}
	export class Snippet {
	    id: string;
	    name: string;
	    command: string;
	    group: string;
	
	    static createFrom(source: any = {}) {
	        return new Snippet(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.command = source["command"];
	        this.group = source["group"];
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
	    encoding: string;
	    hostKeyMode: string;
	    jumpHost: string;
	    jumpPort: number;
	    jumpUser: string;
	    jumpPassword: string;
	    jumpPrivateKeyPath: string;
	    jumpPassphrase: string;
	    proxyType: string;
	    proxyHost: string;
	    proxyPort: number;
	    proxyUser: string;
	    proxyPassword: string;
	    credentialId: string;
	
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
	        this.encoding = source["encoding"];
	        this.hostKeyMode = source["hostKeyMode"];
	        this.jumpHost = source["jumpHost"];
	        this.jumpPort = source["jumpPort"];
	        this.jumpUser = source["jumpUser"];
	        this.jumpPassword = source["jumpPassword"];
	        this.jumpPrivateKeyPath = source["jumpPrivateKeyPath"];
	        this.jumpPassphrase = source["jumpPassphrase"];
	        this.proxyType = source["proxyType"];
	        this.proxyHost = source["proxyHost"];
	        this.proxyPort = source["proxyPort"];
	        this.proxyUser = source["proxyUser"];
	        this.proxyPassword = source["proxyPassword"];
	        this.credentialId = source["credentialId"];
	    }
	}
	export class StoredSession {
	    id: string;
	    name: string;
	    protocol: string;
	    host: string;
	    port: number;
	    username: string;
	    encoding: string;
	    hostKeyMode: string;
	    group: string;
	    proxyType: string;
	    proxyHost: string;
	    proxyPort: number;
	    proxyUser: string;
	    credentialId: string;
	    tags: string[];
	    privateKeyPath: string;
	    otp?: string;
	    jumpHost: string;
	    jumpPort: number;
	    jumpUser: string;
	    jumpPrivateKeyPath: string;
	
	    static createFrom(source: any = {}) {
	        return new StoredSession(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.protocol = source["protocol"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.encoding = source["encoding"];
	        this.hostKeyMode = source["hostKeyMode"];
	        this.group = source["group"];
	        this.proxyType = source["proxyType"];
	        this.proxyHost = source["proxyHost"];
	        this.proxyPort = source["proxyPort"];
	        this.proxyUser = source["proxyUser"];
	        this.credentialId = source["credentialId"];
	        this.tags = source["tags"];
	        this.privateKeyPath = source["privateKeyPath"];
	        this.otp = source["otp"];
	        this.jumpHost = source["jumpHost"];
	        this.jumpPort = source["jumpPort"];
	        this.jumpUser = source["jumpUser"];
	        this.jumpPrivateKeyPath = source["jumpPrivateKeyPath"];
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
	export class UpdateInfo {
	    latestVersion: string;
	    downloadUrl: string;
	    notes: string;
	    hasUpdate: boolean;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.latestVersion = source["latestVersion"];
	        this.downloadUrl = source["downloadUrl"];
	        this.notes = source["notes"];
	        this.hasUpdate = source["hasUpdate"];
	    }
	}

}

export namespace store {
	
	export class AlertConfig {
	    enabled: boolean;
	    cpuThreshold: number;
	    memThreshold: number;
	    diskThreshold: number;
	    webhookUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new AlertConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.cpuThreshold = source["cpuThreshold"];
	        this.memThreshold = source["memThreshold"];
	        this.diskThreshold = source["diskThreshold"];
	        this.webhookUrl = source["webhookUrl"];
	    }
	}
	export class AlertRecord {
	    id: string;
	    time: string;
	    session: string;
	    metric: string;
	    value: number;
	    threshold: number;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new AlertRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.time = source["time"];
	        this.session = source["session"];
	        this.metric = source["metric"];
	        this.value = source["value"];
	        this.threshold = source["threshold"];
	        this.type = source["type"];
	    }
	}
	export class Task {
	    id: string;
	    name: string;
	    sessionId: number;
	    intervalSeconds: number;
	    command: string;
	    enabled: boolean;
	    lastRun: string;
	    lastError: string;
	
	    static createFrom(source: any = {}) {
	        return new Task(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.sessionId = source["sessionId"];
	        this.intervalSeconds = source["intervalSeconds"];
	        this.command = source["command"];
	        this.enabled = source["enabled"];
	        this.lastRun = source["lastRun"];
	        this.lastError = source["lastError"];
	    }
	}

}

