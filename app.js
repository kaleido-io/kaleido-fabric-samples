'use strict';

process.env.HFC_LOGGING='{"info":"console","error":"console"}';
const hasbin = require('hasbin');
const fs = require('fs-extra');
const { join } = require('path');
const prompt = require('prompt-sync')();
const exec = require('child_process').spawn;
const KaleidoClient = require('./lib/kaleido');
const { handleOutput } = require('./lib/utils');

const chaincodeName = process.env.CCNAME || 'asset_transfer';
const userId = process.env.USER_ID || 'user01';

main();

async function writeCAFile(config, membershipId) {
  const fileName = join(__dirname, `tls`, `${membershipId}.pem`);
  const caId = config.organizations[membershipId].certificateAuthorities[0];
  const pem = config.certificateAuthorities[caId].tlsCACerts.pem[0];
  await fs.writeFile(fileName, pem);
  return fileName;
}

async function writeCertFile(kclient, username) {
  const wallet = await kclient.wallet.get(username);
  const fileName = join(__dirname, `tls`, `${username}.pem`);
  await fs.writeFile(fileName, wallet.credentials.certificate);
  return fileName;
}

async function writeKeyFile(kclient, username) {
  const wallet = await kclient.wallet.get(username);
  const fileName = join(__dirname, `tls`, `${username}.key`);
  await fs.writeFile(fileName, wallet.credentials.privateKey);
  return fileName;
}

async function main() {
  const kclient = new KaleidoClient(userId);
  await kclient.init();

  const membership = kclient.myMembership;
  const config = kclient.config;
  const channel = kclient.channel;

  try {
    if (!hasbin.sync('peer')) {
      console.error('Must add "peer" command to system path');
      process.exit(1);
    }

    process.env.FABRIC_CFG_PATH = kclient.userConfigDir;
    process.env.CORE_PEER_TLS_ENABLED = true;
    process.env.CORE_PEER_LOCALMSPID = membership;
    const isInit = prompt('Calling "InitLedger" (y/n)? ');
    const myOrderer = config.organizations[membership].orderers[0];
    const userKeyfile = await writeKeyFile(kclient, 'user01');
    const userCertfile = await writeCertFile(kclient, 'user01');
    const caFile = await writeCAFile(config, membership);
    const args = [
      'chaincode',
      'invoke',
      '--channelID', channel.name,
      '--name', chaincodeName,
      '-o', `${config.orderers[myOrderer].url}`,
      '--tls',
      '--clientauth',
      '--cafile', caFile,
      '--keyfile', userKeyfile,
      '--certfile', userCertfile,
    ];
    for (let member of channel.members) {
      if (!config.organizations[member]) continue;
      const memberPeer = config.organizations[member].peers[0];
      args.push('--peerAddresses');
      args.push(`${config.peers[memberPeer].url}`),
      args.push('--tlsRootCertFiles');
      args.push(await writeCAFile(config, member))
    }
    if (isInit === 'y') {
      args.push('--isInit');
      args.push('-c');
      args.push('{"Args":["InitLedger"]}');
    } else {
      const assetId = prompt('Id of asset to create: ');
      args.push('-c');
      args.push(`{"Args":["CreateAsset", "${assetId}", "yellow", "5", "Tom", "1300"]}`);
    }

    let cmd = exec(
      'peer',
      args,
      {
        cwd: kclient.userConfigDir,
        env: {
          PATH: process.env.PATH,
          CORE_PEER_LOCALMSPID: membership,
          CORE_PEER_TLS_ENABLED: true,
          CORE_PEER_TLS_CLIENTAUTHREQUIRED: true,
          CORE_PEER_TLS_ROOTCERT_FILE: caFile,
          CORE_PEER_TLS_CLIENTCERT_FILE: userCertfile,
          CORE_PEER_TLS_CLIENTKEY_FILE: userKeyfile,
        }
      }
    );
    await handleOutput(cmd);
  } catch(err) {
    console.error(err);
  }
}
