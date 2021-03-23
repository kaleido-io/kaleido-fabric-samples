'use strict';

const prompt = require('prompt-sync')();
const axios = require('axios');
const { join } = require('path');
const os = require('os');
const fs = require('fs-extra');
const { KEYUTIL, KJUR } = require('jsrsasign');
const { Wallets } = require('fabric-network');
const UserWallet = require('./wallet.js');

class KaleidoClient {
  constructor(userId) {
    this.userId = userId;
    const apiKey = process.env.APIKEY;
    if (!apiKey) {
      console.error('Must set environment variable "APIKEY" to proceed');
      process.exit(1);
    }    
    this.apiAuth = {
      headers: {
        Authorization: `Bearer ${apiKey}`
      }
    };
    const url = process.env.KALEIDO_URL || 'https://console.kaleido.io';
    this.kaleidoUrl = `${url}/api/v1`;
  }

  async init() {
    const consortium = await this.locateConsortium();
    const environment = await this.locateEnvironment(consortium);
    const { own, all: memberships } = await this.locateMembership(consortium);
    this.myMembership = own;
    this.channel = await this.locateChannel(consortium, environment, memberships);
    await this.locateFabricCAs(consortium, environment);

    this.userConfigDir = join(os.homedir(), 'fabric-test', this.myMembership, environment, this.userId);
    this.wallet = await this.ensureUserWallet(consortium, environment, own);    
    this.config = await this.buildNetworkConfig(consortium, environment, memberships);
  }

  async locateConsortium() {
    let result = await axios.get(`${this.kaleidoUrl}/c`, this.apiAuth );
    let consortium;
    if (result.data.length === 0) {
      console.log('No business network found in the Kaleido account');
      process.exit(1);
    } else if (result.data.length > 1) {
      console.log('Found these business networks:');
      let i = 0;
      for (let network of result.data) {
        console.log(`\t[${i++}] id: ${network._id}, name: ${network.name}`);
      }
      const consortiumId = prompt('Select target consortium: ');
      consortium = result.data[consortiumId]._id;
    } else {
      console.log(`Found business network "${result.data[0].name}" (${result.data[0]._id})`);
      consortium = result.data[0]._id;
    }
    return consortium;
  }
  
  async locateMembership(consortiumId) {
    let result = await axios.get(`${this.kaleidoUrl}/c/${consortiumId}/m`, this.apiAuth);
    let membership;
    if (result.data.length === 0) {
      console.error(`No memberships found in the business network ${consortiumId}`);
      process.exit(1);
    } else if (result.data.length > 1) {
      console.log('Found these memberships:');
      let i = 0;
      for (let member of result.data) {
        console.log(`\t[${i++}] id: ${member._id}, name: ${member.org_name}`);
      }
      const membershipId = prompt('Select membership to use to submit transactions: ');
      membership = result.data[membershipId]._id;
    } else {
      console.log(`Found membership "${result.data[0].org_name}" (${result.data[0]._id})`);
      membership = result.data[0]._id;
    }
    return {
      own: membership,
      all: result.data
    };
  }
  
  async locateEnvironment(consortiumId) {
    let result = await axios.get(`${this.kaleidoUrl}/c/${consortiumId}/e`, this.apiAuth);
    let env;
    if (result.data.length === 0) {
      console.error(`No environments found in the business network ${consortiumId}`);
      process.exit(1);
    } else if (result.data.length > 1) {
      console.log('Found these environments:');
      let i = 0;
      for (let network of result.data) {
        console.log(`\t[${i++}] id: ${network._id}, name: ${network.name}`);
      }
      const environmentId = prompt('Select target environment: ');
      env = result.data[environmentId]._id;
    } else {
      console.log(`Found environment "${result.data[0].name}" (${result.data[0]._id})`);
      env = result.data[0]._id;
    }
    return env;
  }
  
  async locateChannel(consortiumId, environmentId, memberships) {
    let result = await axios.get(`${this.kaleidoUrl}/c/${consortiumId}/e/${environmentId}/channels`, this.apiAuth);
    let ret;
    if (result.data.length === 0) {
      console.info(`No user-created channels found in the environment ${environmentId}, using "default-channel"`);
      return { name: 'default-channel', members: memberships.map(m => m._id) };
    } else if (result.data.length > 0) {
      console.log('Found these channels:');
      const channels = [{ name: 'default-channel' }].concat(result.data);
      let i = 0;
      for (let channel of channels) {
        console.log(`\t[${i++}] name: ${channel.name}`);
      }
      const channel = await prompt('Select channel: ');
      ret = channels[channel];
    }
    return ret;
  }

  // a user wallet contains both MSP materials so it can be used with fabric commands (peer, osnadmin etc.)
  // and a Wallet as designed by the fabric-network module
  async ensureUserWallet() {
    const userMspPath = join(this.userConfigDir, 'msp');
    let wallet;
    try {
      fs.accessSync(userMspPath, fs.F_OK);
      wallet = await Wallets.newFileSystemWallet(this.userConfigDir);
    } catch(err) {
      const caCertPEM = await this.getCACert();
      const { csrPEM, keyPEM } = await this.generateCSR();
      const secret = await this.registerNewUser();
      const userwallet = new UserWallet(this.userId);
      const certPEM = await userwallet.signCert(csrPEM, secret, this.cas[this.myMembership].url);
      userwallet.createMSPDir(this.userConfigDir, caCertPEM, keyPEM, certPEM, this.myMembership);
      wallet = await userwallet.createUserWallet(this.userConfigDir, keyPEM, certPEM, this.myMembership);
      console.log(`Created user ${this.userId} MSP materials in dir ${this.userConfigDir}`);
    }
    
    return wallet;
  }
  
  async locateFabricCAs(consortiumId, envId) {
    let result = await axios.get(`${this.kaleidoUrl}/c/${consortiumId}/e/${envId}/s`, this.apiAuth);
    if (result.data.length === 0) {
      console.error(`No services found in the environment ${envId}`);
      process.exit(1);
    } else if (result.data.length >= 1) {
      const fabcas = result.data.filter(s => s.service === 'fabric-ca');
      if (fabcas.length === 0) {
        console.error(`No fabric-ca services found`);
        process.exit(1);
      } else {
        console.log(`Found Fabric CAs:`);
        this.cas = {};
        for (let ca of fabcas) {
          console.log(`\tid: ${ca._id}, membership: ${ca.membership_id}`);
          this.cas[ca.membership_id] = {
            url: ca.urls.http,
            id: ca._id
          };
        }
      }
    }
  }
  
  async getCACert() {
    const result = await axios.get(`${this.kaleidoUrl}/fabric-ca/${this.cas[this.myMembership].id}/cacert`, this.apiAuth);
    return result.data.cert;
  }
  
  async generateCSR() {
    let subject = `/C=US/L=Raleigh/O=Kaleido/OU=admin/CN=${this.userId}`;
    const alg = 'EC';
    const keylenOrCurve = "secp256r1";
    const sigalgName = 'SHA256withECDSA';
    let keypair = KEYUTIL.generateKeypair(alg, keylenOrCurve);
  
    let options = {
      sigalg: sigalgName,
      subject: {str: subject},
      sbjpubkey: keypair.pubKeyObj,
      sbjprvkey: keypair.prvKeyObj
    };
  
    let csrPEM = KJUR.asn1.csr.CSRUtil.newCSRPEM(options);
    let keyPEM = KEYUTIL.getPEM(keypair.prvKeyObj, "PKCS8PRV");
  
    return { csrPEM, keyPEM };
  }

  async registerNewUser() {
    let result = await axios.post(`${this.kaleidoUrl}/fabric-ca/${this.cas[this.myMembership].id}/register`, {
      registrations: [{
        enrollmentID: this.userId,
        role: 'admin'
      }]
    }, this.apiAuth);
    return result.data.registrations[0].enrollmentSecret;
  }
  
  async buildNetworkConfig(consortiumId, envId, memberships) {
    const config = {
      client: {
        organization: this.myMembership,
        connection: {
          timeout: {
            peer: {
              endorser: '300'
            }
          }
        }
      },
      organizations: {},
      peers: {},
      orderers: {},
      certificateAuthorities: {}
    };
    for (let membership of memberships) {
      try {
        const membershipId = membership._id;
        const {url: ordererUrl, id: ordererId, caCertPEM} = await this.locateOrderer(consortiumId, envId, membershipId);
        const {url: peerUrl, id: peerId} = await this.locatePeer(consortiumId, envId, membershipId);
        config.organizations[membershipId] = {
          mspid: membershipId,
          peers: [ peerId ],
          orderers: [ ordererId ],
          certificateAuthorities: [this.cas[membershipId].id]
        };
        config.peers[peerId] = {
          url: peerUrl,
          tlsCACerts: {
            pem: caCertPEM
          }
        };
        config.orderers[ordererId] = {
          url: ordererUrl,
          tlsCACerts: {
            pem: caCertPEM
          }
        };
        config.certificateAuthorities[this.cas[membershipId].id] = {
          url: this.cas[membershipId].url,
          caName: "",
          tlsCACerts: {
            pem: [caCertPEM]
          },
          httpOptions: {
            verify: false
          }
        }
      }
      catch(err) {
        console.log(`Skipping membership '${membership.name}' [${membership._id}]: ${err}`);
      }
    }
    return config;
  }
  
  async locateOrderer(consortiumId, envId, membershipId) {
    let result = await axios.get(`${this.kaleidoUrl}/c/${consortiumId}/e/${envId}/n`, this.apiAuth);
    let orderer;
    if (result.data.length === 0) {
      console.error(`No orderers found in the environment ${envId}`);
      throw new Error(`No orderers found in the environment ${envId}`)
    } else if (result.data.length >= 1) {
      console.log(`Found these orderers for the membership:`);
      const orderersForMembership = result.data.filter(a => a.membership_id === membershipId && a.role === 'orderer');
      for (let orderer of orderersForMembership) {
        console.log(`\tid: ${orderer._id}, name: ${orderer.name}`);
      }
      if (orderersForMembership.length === 0) {
        throw new Error(`No orderers found for the membership ${membershipId}`)
      } else if (orderersForMembership.length > 1) {
        const ordererId = prompt('Select orderer: ');
        orderer = orderersForMembership.find(o => o._id === ordererId);
      } else {
        console.log(`Found orderer for the membership: ${orderersForMembership[0]._id}`);
        orderer = orderersForMembership[0];
      }
    }
    const {orgCA: caCertPEM} = JSON.parse(Buffer.from(orderer.node_identity_data, 'hex').toString());
    return {
      url: `${orderer.urls.orderer.slice('https://'.length)}:443`,
      id: orderer._id,
      caCertPEM
    };
  }
  
  async locatePeer(consortiumId, envId, membershipId) {
    let result = await axios.get(`${this.kaleidoUrl}/c/${consortiumId}/e/${envId}/n`, this.apiAuth);
    let peer;
    if (result.data.length === 0) {
      console.error(`No peers found in the environment ${envId}`);
      process.exit(1);
    } else if (result.data.length >= 1) {
      console.log(`Found these peers for the membership:`);
      const peersForMembership = result.data.filter(a => a.membership_id === membershipId && a.role === 'peer');
      for (let peer of peersForMembership) {
        console.log(`\tid: ${peer._id}, name: ${peer.name}`);
      }
      if (peersForMembership.length === 0) {
        console.error(`No peers found for the membership ${membershipId}`);
        process.exit(1);
      } else if (peersForMembership.length > 1) {
        const peerId = prompt('Select peer: ');
        peer = peersForMembership.find(o => o._id === peerId);
      } else {
        console.log(`Found peer for the membership: ${peersForMembership[0]._id}`);
        peer = peersForMembership[0];
      }
    }
    return {
      url: `${peer.urls.peer.slice('https://'.length)}:443`,
      id: peer._id
    };
  }
}

module.exports = KaleidoClient;