export namespace app {
	
	export class AccountInfo {
	    index: number;
	    address: string;
	
	    static createFrom(source: any = {}) {
	        return new AccountInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.address = source["address"];
	    }
	}
	export class NodeStatus {
	    mode: string;
	    connected: boolean;
	    syncing: boolean;
	    height: number;
	    peers: number;
	
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
	    }
	}
	export class SendPreview {
	    toAddress: string;
	    symbol: string;
	    zts: string;
	    amount: string;
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
	export class Settings {
	    nodeUrl: string;
	    theme: string;
	    lastWallet: string;
	    activeAccount: number;
	    allowMainnetSend: boolean;
	    autoReceive: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nodeUrl = source["nodeUrl"];
	        this.theme = source["theme"];
	        this.lastWallet = source["lastWallet"];
	        this.activeAccount = source["activeAccount"];
	        this.allowMainnetSend = source["allowMainnetSend"];
	        this.autoReceive = source["autoReceive"];
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
	export class TxRecord {
	    hash: string;
	    direction: string;
	    counterparty: string;
	    token: string;
	    amount: string;
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
	        this.counterparty = source["counterparty"];
	        this.token = source["token"];
	        this.amount = source["amount"];
	        this.momentumHeight = source["momentumHeight"];
	        this.confirmed = source["confirmed"];
	        this.timestamp = source["timestamp"];
	    }
	}
	export class UnreceivedBlock {
	    fromHash: string;
	    fromAddress: string;
	    token: string;
	    amount: string;
	
	    static createFrom(source: any = {}) {
	        return new UnreceivedBlock(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fromHash = source["fromHash"];
	        this.fromAddress = source["fromAddress"];
	        this.token = source["token"];
	        this.amount = source["amount"];
	    }
	}
	export class WalletMeta {
	    name: string;
	    baseAddress: string;
	
	    static createFrom(source: any = {}) {
	        return new WalletMeta(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.baseAddress = source["baseAddress"];
	    }
	}

}

