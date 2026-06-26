export namespace app {
	
	export class AccountInfo {
	    index: number;
	    address: string;
	    label: string;
	
	    static createFrom(source: any = {}) {
	        return new AccountInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.address = source["address"];
	        this.label = source["label"];
	    }
	}
	export class CallPreview {
	    toAddress: string;
	    zts: string;
	    symbol: string;
	    amount: string;
	    decimals: number;
	    hash: string;
	    summary: string;
	    usedPlasma: number;
	    difficulty: number;
	    needsPoW: boolean;
	
	    static createFrom(source: any = {}) {
	        return new CallPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.toAddress = source["toAddress"];
	        this.zts = source["zts"];
	        this.symbol = source["symbol"];
	        this.amount = source["amount"];
	        this.decimals = source["decimals"];
	        this.hash = source["hash"];
	        this.summary = source["summary"];
	        this.usedPlasma = source["usedPlasma"];
	        this.difficulty = source["difficulty"];
	        this.needsPoW = source["needsPoW"];
	    }
	}
	export class Contact {
	    name: string;
	    address: string;
	
	    static createFrom(source: any = {}) {
	        return new Contact(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.address = source["address"];
	    }
	}
	export class DelegationInfo {
	    name: string;
	    status: number;
	    weight: string;
	
	    static createFrom(source: any = {}) {
	        return new DelegationInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.status = source["status"];
	        this.weight = source["weight"];
	    }
	}
	export class EmbeddedInfo {
	    running: boolean;
	    dataDir: string;
	    sizeBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new EmbeddedInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.dataDir = source["dataDir"];
	        this.sizeBytes = source["sizeBytes"];
	    }
	}
	export class FusionEntry {
	    id: string;
	    beneficiary: string;
	    qsrAmount: string;
	    expirationHeight: number;
	    isRevocable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FusionEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.beneficiary = source["beneficiary"];
	        this.qsrAmount = source["qsrAmount"];
	        this.expirationHeight = source["expirationHeight"];
	        this.isRevocable = source["isRevocable"];
	    }
	}
	export class NodeConfig {
	    mode: string;
	    remoteUrl: string;
	    localUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new NodeConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.remoteUrl = source["remoteUrl"];
	        this.localUrl = source["localUrl"];
	    }
	}
	export class NodeStatus {
	    mode: string;
	    connected: boolean;
	    syncing: boolean;
	    height: number;
	    peers: number;
	    chainId: number;
	
	    static createFrom(source: any = {}) {
	        return new NodeStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.connected = source["connected"];
	        this.syncing = source["syncing"];
	        this.height = source["height"];
	        this.peers = source["peers"];
	        this.chainId = source["chainId"];
	    }
	}
	export class VoteBreakdownDTO {
	    total: number;
	    yes: number;
	    no: number;
	
	    static createFrom(source: any = {}) {
	        return new VoteBreakdownDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.yes = source["yes"];
	        this.no = source["no"];
	    }
	}
	export class PhaseDTO {
	    id: string;
	    projectId: string;
	    name: string;
	    description: string;
	    url: string;
	    znnFundsNeeded: string;
	    qsrFundsNeeded: string;
	    creationTimestamp: number;
	    acceptedTimestamp: number;
	    status: number;
	    votes: VoteBreakdownDTO;
	
	    static createFrom(source: any = {}) {
	        return new PhaseDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.projectId = source["projectId"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.url = source["url"];
	        this.znnFundsNeeded = source["znnFundsNeeded"];
	        this.qsrFundsNeeded = source["qsrFundsNeeded"];
	        this.creationTimestamp = source["creationTimestamp"];
	        this.acceptedTimestamp = source["acceptedTimestamp"];
	        this.status = source["status"];
	        this.votes = this.convertValues(source["votes"], VoteBreakdownDTO);
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
	export class PillarSummary {
	    name: string;
	    rank: number;
	    weight: string;
	    delegateRewardPercent: number;
	    producerAddress: string;
	
	    static createFrom(source: any = {}) {
	        return new PillarSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.rank = source["rank"];
	        this.weight = source["weight"];
	        this.delegateRewardPercent = source["delegateRewardPercent"];
	        this.producerAddress = source["producerAddress"];
	    }
	}
	export class PlasmaInfo {
	    qsrFused: string;
	    currentPlasma: number;
	    maxPlasma: number;
	
	    static createFrom(source: any = {}) {
	        return new PlasmaInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.qsrFused = source["qsrFused"];
	        this.currentPlasma = source["currentPlasma"];
	        this.maxPlasma = source["maxPlasma"];
	    }
	}
	export class ProjectDTO {
	    id: string;
	    owner: string;
	    name: string;
	    description: string;
	    url: string;
	    znnFundsNeeded: string;
	    qsrFundsNeeded: string;
	    creationTimestamp: number;
	    lastUpdateTimestamp: number;
	    status: number;
	    votes: VoteBreakdownDTO;
	    phases: PhaseDTO[];
	
	    static createFrom(source: any = {}) {
	        return new ProjectDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.owner = source["owner"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.url = source["url"];
	        this.znnFundsNeeded = source["znnFundsNeeded"];
	        this.qsrFundsNeeded = source["qsrFundsNeeded"];
	        this.creationTimestamp = source["creationTimestamp"];
	        this.lastUpdateTimestamp = source["lastUpdateTimestamp"];
	        this.status = source["status"];
	        this.votes = this.convertValues(source["votes"], VoteBreakdownDTO);
	        this.phases = this.convertValues(source["phases"], PhaseDTO);
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
	export class ProjectListDTO {
	    count: number;
	    list: ProjectDTO[];
	
	    static createFrom(source: any = {}) {
	        return new ProjectListDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.count = source["count"];
	        this.list = this.convertValues(source["list"], ProjectDTO);
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
	export class RewardInfo {
	    znn: string;
	    qsr: string;
	
	    static createFrom(source: any = {}) {
	        return new RewardInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.znn = source["znn"];
	        this.qsr = source["qsr"];
	    }
	}
	export class SendPreview {
	    toAddress: string;
	    symbol: string;
	    zts: string;
	    amount: string;
	    decimals: number;
	    usedPlasma: number;
	    difficulty: number;
	    hash: string;
	    needsPoW: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SendPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.toAddress = source["toAddress"];
	        this.symbol = source["symbol"];
	        this.zts = source["zts"];
	        this.amount = source["amount"];
	        this.decimals = source["decimals"];
	        this.usedPlasma = source["usedPlasma"];
	        this.difficulty = source["difficulty"];
	        this.hash = source["hash"];
	        this.needsPoW = source["needsPoW"];
	    }
	}
	export class SendRequest {
	    toAddress: string;
	    zts: string;
	    amount: string;
	
	    static createFrom(source: any = {}) {
	        return new SendRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.toAddress = source["toAddress"];
	        this.zts = source["zts"];
	        this.amount = source["amount"];
	    }
	}
	export class SentinelInfo {
	    owner: string;
	    registrationTimestamp: number;
	    isRevocable: boolean;
	    revokeCooldown: number;
	    active: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SentinelInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.owner = source["owner"];
	        this.registrationTimestamp = source["registrationTimestamp"];
	        this.isRevocable = source["isRevocable"];
	        this.revokeCooldown = source["revokeCooldown"];
	        this.active = source["active"];
	    }
	}
	export class Settings {
	    nodeUrl?: string;
	    nodeMode: string;
	    remoteNodeUrl: string;
	    localNodeUrl: string;
	    theme: string;
	    lastWallet: string;
	    activeAccount: number;
	    allowMainnetSend: boolean;
	    chainId: number;
	    autoReceive: boolean;
	    accountLabels: Record<string, string>;
	    accountCounts: Record<string, number>;
	    contacts: Contact[];
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nodeUrl = source["nodeUrl"];
	        this.nodeMode = source["nodeMode"];
	        this.remoteNodeUrl = source["remoteNodeUrl"];
	        this.localNodeUrl = source["localNodeUrl"];
	        this.theme = source["theme"];
	        this.lastWallet = source["lastWallet"];
	        this.activeAccount = source["activeAccount"];
	        this.allowMainnetSend = source["allowMainnetSend"];
	        this.chainId = source["chainId"];
	        this.autoReceive = source["autoReceive"];
	        this.accountLabels = source["accountLabels"];
	        this.accountCounts = source["accountCounts"];
	        this.contacts = this.convertValues(source["contacts"], Contact);
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
	export class StakeEntry {
	    id: string;
	    amount: string;
	    startTimestamp: number;
	    expirationTimestamp: number;
	    durationMonths: number;
	    isMatured: boolean;
	
	    static createFrom(source: any = {}) {
	        return new StakeEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.amount = source["amount"];
	        this.startTimestamp = source["startTimestamp"];
	        this.expirationTimestamp = source["expirationTimestamp"];
	        this.durationMonths = source["durationMonths"];
	        this.isMatured = source["isMatured"];
	    }
	}
	export class StakeInfo {
	    totalAmount: string;
	    entries: StakeEntry[];
	
	    static createFrom(source: any = {}) {
	        return new StakeInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalAmount = source["totalAmount"];
	        this.entries = this.convertValues(source["entries"], StakeEntry);
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
	export class TokenBalance {
	    zts: string;
	    symbol: string;
	    decimals: number;
	    amount: string;
	
	    static createFrom(source: any = {}) {
	        return new TokenBalance(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.zts = source["zts"];
	        this.symbol = source["symbol"];
	        this.decimals = source["decimals"];
	        this.amount = source["amount"];
	    }
	}
	export class TokenInfo {
	    name: string;
	    symbol: string;
	    domain: string;
	    tokenStandard: string;
	    owner: string;
	    totalSupply: string;
	    maxSupply: string;
	    decimals: number;
	    isMintable: boolean;
	    isBurnable: boolean;
	    isUtility: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TokenInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.symbol = source["symbol"];
	        this.domain = source["domain"];
	        this.tokenStandard = source["tokenStandard"];
	        this.owner = source["owner"];
	        this.totalSupply = source["totalSupply"];
	        this.maxSupply = source["maxSupply"];
	        this.decimals = source["decimals"];
	        this.isMintable = source["isMintable"];
	        this.isBurnable = source["isBurnable"];
	        this.isUtility = source["isUtility"];
	    }
	}
	export class TxRecord {
	    hash: string;
	    direction: string;
	    method: string;
	    counterparty: string;
	    token: string;
	    amount: string;
	    decimals: number;
	    momentumHeight: number;
	    confirmed: boolean;
	    timestamp: number;
	
	    static createFrom(source: any = {}) {
	        return new TxRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hash = source["hash"];
	        this.direction = source["direction"];
	        this.method = source["method"];
	        this.counterparty = source["counterparty"];
	        this.token = source["token"];
	        this.amount = source["amount"];
	        this.decimals = source["decimals"];
	        this.momentumHeight = source["momentumHeight"];
	        this.confirmed = source["confirmed"];
	        this.timestamp = source["timestamp"];
	    }
	}
	export class TxPage {
	    records: TxRecord[];
	    hasMore: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TxPage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.records = this.convertValues(source["records"], TxRecord);
	        this.hasMore = source["hasMore"];
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
	
	export class UnreceivedBlock {
	    fromHash: string;
	    fromAddress: string;
	    token: string;
	    amount: string;
	    decimals: number;
	
	    static createFrom(source: any = {}) {
	        return new UnreceivedBlock(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fromHash = source["fromHash"];
	        this.fromAddress = source["fromAddress"];
	        this.token = source["token"];
	        this.amount = source["amount"];
	        this.decimals = source["decimals"];
	    }
	}
	
	export class WalletMeta {
	    id: string;
	    name: string;
	    baseAddress: string;
	
	    static createFrom(source: any = {}) {
	        return new WalletMeta(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.baseAddress = source["baseAddress"];
	    }
	}

}

