<?php

/*
 * To test:
 *
 * make
 * vpn-ca init
 * vpn-ca -server vpn-daemon
 * vpn-ca -client vpn-daemon-client
 * ./_bin/vpn-daemon &
 * php php/vpn-daemon-client-tls.php
 */

$commandList = [
    'SET_PORTS 11940 11941',
    'LIST',
    'DISCONNECT foo bar baz',
//  'SETUP cn profile1 profile2',
    'QUIT',
];

// @see https://www.php.net/manual/en/context.ssl.php
// @see https://www.php.net/manual/en/transports.inet.php
$streamContext = stream_context_create(
    [
        'ssl' => [
            'peer_name' => 'vpn-daemon',
            'cafile' => dirname(__DIR__).'/ca.crt',
            'local_cert' => dirname(__DIR__).'/client/vpn-daemon-client.crt',
            'local_pk' => dirname(__DIR__).'/client/vpn-daemon-client.key',
        ],
    ]
);

$socket = stream_socket_client('ssl://localhost:41194', $errno, $errstr, 5, STREAM_CLIENT_CONNECT, $streamContext);

foreach ($commandList as $cmd) {
    var_dump(sendCommand($socket, $cmd));
}

/*
$localCommandList = [
    'CLIENT_CONNECT profile1 cn 1234567890 10.52.58.2 fdbf:4dff:a892:1572::1000'
    'CLIENT_DISCONNECT profile1 cn 1234567890 10.52.58.2 fdbf:4dff:a892:1572::1000 605666 9777056 120'
];

$localSocket = stream_socket_client('tcp://localhost:41195');

foreach ($localCommandList as $cmd) {
    var_dump(sendCommand($localSocket, $cmd));
}
*/

function sendCommand($socket, $cmd)
{
    fwrite($socket, sprintf("%s\n", $cmd));

    return handleResponse($socket);
}

function handleResponse($socket)
{
    $statusLine = fgets($socket);
    if (0 !== strpos($statusLine, 'OK: ')) {
        echo $statusLine;
        exit(1);
    }
    $resultLineCount = (int) substr($statusLine, 4);
    $resultData = [];
    for ($i = 0; $i < $resultLineCount; $i++) {
        $resultData[] = trim(fgets($socket));
    }
   
    return $resultData;
}
