<?php

namespace Runner;

final class RPCResponse {
    public function __construct(
        public int $id,
        public mixed $result,
        public ?string $error = null,
    ) {
    }

    public static function deserialize(string $response) {
        $decoded = json_decode($response, true);

        return new RPCResponse(
            $decoded['id'],
            $decoded['result'],
            $decoded['error'] ?? null
        );
    }
}
