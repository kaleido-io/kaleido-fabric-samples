'use strict';

const prompt = require('prompt-sync')();
const axios = require('axios');
const { join } = require('path');
const os = require('os');
const fs = require('fs-extra');
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
    const url = process.env.KALEIDO_URL || 'https://console.kaleido.io/api/v1';
    this.kaleidoUrl = `${url}/api/v1`;
  }

  async init() {
    const consortium = await this.locateConsortium();
    const environment = await this.locateEnvironment(consortium);
    const { own, all: memberships } = await this.locateMembership(consortium);
    this.myMembership = own;
    this.channel = await this.locateChannel(consortium, environment, memberships);
    await this.locateFabricCAs(consortium, environment);
    await this.locateOrderers(consortium, environment);
    await this.locatePeers(consortium, environment);

    this.walletDir = join(os.homedir(), 'fabric-test', environment);
    this.memberCaDir = join(os.homedir(), 'fabric-test', environment, this.myMembership, 'membercas');
    await fs.ensureDir(this.memberCaDir);

    this.wallet = new UserWallet(this.walletDir, this.myMembership);
    await this.wallet.init();
    let user = await this.wallet.getUser(this.userId);
    if (!user) {
      const secret = await this.registerNewUser();
      user = await this.wallet.newUser(this.userId, secret, this.cas[this.myMembership].url);
    }
    this.user = user;

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
      if (process.env.MEMBERSHIP) {
        membership = process.env.MEMBERSHIP;
      } else {
        const membershipId = prompt('Select membership to use to submit transactions: ');
        membership = result.data[membershipId]._id;
      }
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
  
  async locateChannel(consortiumId, environmentId) {
    let result = await axios.get(`${this.kaleidoUrl}/c/${consortiumId}/e/${environmentId}/channels`, this.apiAuth);
    let ret;
    if (result.data.length === 0) {
      console.info(`No channels found in the environment ${environmentId}. The "default-channel" should have been created. Something went wrong in the environment. Exiting.`);
      process.exit(1);
    } else if (result.data.length > 1) {
      console.log('Found these channels:');
      const channels = result.data;
      let i = 0;
      for (let channel of channels) {
        console.log(`\t[${i++}] name: ${channel.name}`);
      }
      const channel = await prompt('Select channel: ');
      ret = channels[channel];
    } else if (result.data.length === 1) {
      ret = result.data[0];
    }
    return ret;
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
  
  
  async locateOrderers(consortiumId, envId) {
    let result = await axios.get(`${this.kaleidoUrl}/c/${consortiumId}/e/${envId}/n`, this.apiAuth);
    let orderers = {};
    if (result.data.length === 0) {
      console.error(`No orderers found in the environment ${envId}`);
      throw new Error(`No orderers found in the environment ${envId}`)
    } else {
      console.log(`Found these orderers:`);
      const orderers = result.data.filter(a => a.role === 'orderer');
      this.orderers = orderers.map(orderer => {
        console.log(`\tid: ${orderer._id}, name: ${orderer.name}`);
        const {orgCA: caCertPEM} = JSON.parse(Buffer.from(orderer.node_identity_data, 'hex').toString());
        return {
          hostname: `${orderer.urls.orderer.slice('https://'.length)}`,
          id: orderer._id,
          membershipId: orderer.membership_id,
          caCertPEM
        };
      })
    }
  }
  
  async locatePeers(consortiumId, envId) {
    let result = await axios.get(`${this.kaleidoUrl}/c/${consortiumId}/e/${envId}/n`, this.apiAuth);
    let peer;
    if (result.data.length === 0) {
      console.error(`No peers found in the environment ${envId}`);
      process.exit(1);
    } else {
      console.log(`Found these peers:`);
      const peers = result.data.filter(a => a.role === 'peer');
      this.peers = peers.map(peer => {
        console.log(`\tid: ${peer._id}, name: ${peer.name}`);
        const {orgCA: caCertPEM} = JSON.parse(Buffer.from(peer.node_identity_data, 'hex').toString());
        return {
          hostname: `${peer.urls.peer.slice('https://'.length)}`,
          id: peer._id,
          membershipId: peer.membership_id,
          caCertPEM
        };
      });
    }
  }

  async getCACert() {
    const result = await axios.get(`${this.kaleidoUrl}/fabric-ca/${this.cas[this.myMembership].id}/cacert`, this.apiAuth);
    return result.data.cert;
  }
  
  async registerNewUser() {
    let result = await axios.post(`${this.kaleidoUrl}/fabric-ca/${this.cas[this.myMembership].id}/register`, {
      registrations: [{
        enrollmentID: this.userId,
        role: 'admin'
      }]
    }, this.apiAuth);
    if (result.data.errors && result.data.errors.length > 0) {
      console.log(`Failed to register user ${this.userId}`, result.data.errors);
      process.exit(1);
    }
    return result.data.registrations[0].enrollmentSecret;
  }

  async getCertFile(membershipId) {
    const fileName = join(this.memberCaDir, `${membershipId}.pem`);
    const caId = this.config.organizations[membershipId].certificateAuthorities[0];
    const pem = this.config.certificateAuthorities[caId].tlsCACerts.pem[0];
    await fs.writeFile(fileName, pem);
    return fileName;
  }
  
  async buildNetworkConfig(consortiumId, envId, memberships) {
    let config = {
      client: {
        organization: this.myMembership,
        connection: {
          timeout: {
            peer: {
              endorser: 30,
              committer: 30
            },
            orderer: 30
          }
        }
      },
      channels: {
        [this.channel.name]: {
          orderers:[],
          peers: []
        }
      },
      organizations: {},
      peers: {},
      orderers: {},
      certificateAuthorities: {}
    };
    for (let membership of this.channel.members) {
      const orderers = this.orderers.filter(orderer => orderer.membershipId === membership);
      config.channels[this.channel.name].orderers = config.channels[this.channel.name].orderers.concat(orderers.map(o => o.id));

      const peers = this.peers.filter(peer => peer.membershipId === membership);
      config.channels[this.channel.name].peers = config.channels[this.channel.name].peers.concat(peers.map(p => p.id));
    }
    for (let membership of memberships) {
      const membershipId = membership._id;
      config.organizations[membershipId] = {
        mspid: membershipId,
        peers: this.peers.filter(p => p.membershipId === membershipId).map(p => p.id),
        orderers: this.orderers.filter(o => o.membershipId === membershipId).map(o => o.id),
        certificateAuthorities: [this.cas[membershipId].id]
      };
    }
    for (let peer of this.peers) {
      config.peers[peer.id] = {
        url: `grpcs://${peer.hostname}:443`,
        tlsCACerts: {
          pem: peer.caCertPEM
        }
      }
    }
    for (let orderer of this.orderers) {
      config.orderers[orderer.id] = {
        url: `grpcs://${orderer.hostname}:443`,
        tlsCACerts: {
          pem: orderer.caCertPEM
        }
      }
    }
    // config.certificateAuthorities[this.cas[membershipId].id] = {
    //   url: this.cas[membershipId].url,
    //   caName: "",
    //   tlsCACerts: {
    //     pem: [caCertPEM]
    //   },
    //   httpOptions: {
    //     verify: false
    //   }
    // }

    console.log(JSON.stringify(config, null, 2));
    return config;
  }
}

module.exports = KaleidoClient;