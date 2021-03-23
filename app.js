'use strict';

process.env.HFC_LOGGING='{"info":"console","error":"console"}';
const hasbin = require('hasbin');
const fs = require('fs-extra');
const { join } = require('path');
const prompt = require('prompt-sync')();
const exec = require('child_process').spawn;
const { Gateway } = require('fabric-network');
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
    const args = [
      'chaincode',
      'invoke',
      '--channelID', channel.name,
      '--name', chaincodeName,
      '-o', `${config.orderers[myOrderer].url}`,
      '--tls',
      '--cafile', await writeCAFile(config, membership),
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
        cwd: kclient.userConfigDir
      }
    );
    await handleOutput(cmd);
  } catch(err) {
    console.error(err);
  }
}
