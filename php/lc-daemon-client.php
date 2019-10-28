<?php

/*
 * To test:
 *
 * vpn-ca init
 * vpn-ca -server lc-daemon
 * vpn-ca -client lc-client
 * ./_bin/lc-daemon &
 * php php/lc-daemon-client.php
 */

$commandList = [
    'SET_PORTS 11940 11941',
    'LIST',
    'DISCONNECT foo',
    'DISCONNECT bar',
    'DISCONNECT baz',
    'QUIT',
];

// @see https://www.php.net/manual/en/context.ssl.php
// @see https://www.php.net/manual/en/transports.inet.php
$streamContext = stream_context_create(
    [
        'ssl' => [
            'peer_name' => 'lc-daemon',
            'cafile' => dirname(__DIR__).'/ca.crt',
            'local_cert' => dirname(__DIR__).'/client/lc-client.crt',
            'local_pk' => dirname(__DIR__).'/client/lc-client.key',
        ],
    ]
);

$socket = stream_socket_client('ssl://localhost:41194', $errno, $errstr, 5, STREAM_CLIENT_CONNECT, $streamContext);

foreach ($commandList as $cmd) {
    var_dump(sendCommand($socket, $cmd));
}

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
