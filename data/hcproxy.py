# A minimal proxy implementing only HTTP CONNECT (H-C-proxy).
# Uses the Python standard library only.

import os
import asyncio
import http
import logging
from typing import Any

ENCODING = "latin1"
READ_SIZE = 2048


class HCProxy:
    def __init__(self, port: int) -> None:
        self.port = port
        self.server = None

    def writer_info(self, writer: asyncio.StreamWriter) -> Any:
        return writer.get_extra_info("peername")

    async def send(self, writer: asyncio.StreamWriter, data: bytes = None) -> None:
        writer.write(data)
        await writer.drain()

    async def http_response(
        self, writer: asyncio.StreamWriter, status: http.HTTPStatus
    ) -> None:
        response = f"HTTP/1.1 {status.value} {status.description}\r\n\r\n"
        await self.send(writer, response.encode(ENCODING))

    async def close(self, writer: asyncio.StreamWriter) -> None:
        try:
            writer.close()
            await writer.wait_closed()
        except Exception:
            pass

    def parse_connect(self, line: str) -> tuple[str, int]:
        parts = line.split()
        if len(parts) != 3 or parts[0] != "CONNECT":
            raise Exception("invalid CONNECT request line")

        target = parts[1].split(":")
        return target[0], int(target[1])

    async def copy_stream(
        self, reader: asyncio.StreamReader, writer: asyncio.StreamWriter
    ) -> None:
        while True:
            data = await reader.read(READ_SIZE)
            if not data:
                raise EOFError()
            writer.write(data)
            await writer.drain()

    async def handler_internal(
        self, reader: asyncio.StreamReader, writer: asyncio.StreamWriter
    ) -> None:
        logging.info(f"connection from {self.writer_info(writer)}")

        data = await reader.readline()
        try:
            host, port = self.parse_connect(data.decode(ENCODING).rstrip())
        except Exception:
            await self.http_response(writer, http.HTTPStatus.BAD_REQUEST)
            raise

        while True:
            data = await reader.readline()
            line = data.decode(ENCODING).rstrip()
            if not line:
                break

        r_reader, r_writer = await asyncio.open_connection(host=host, port=port)
        logging.info(
            f"created outgoing connection to {host} {self.writer_info(writer)}"
        )

        await self.http_response(writer, http.HTTPStatus.OK)

        try:
            await asyncio.gather(
                self.copy_stream(reader, r_writer), self.copy_stream(r_reader, writer)
            )
        finally:
            logging.info(
                f"closing outgoing connection to {host} {self.writer_info(writer)}"
            )
            await self.close(r_writer)

    async def handler(
        self, reader: asyncio.StreamReader, writer: asyncio.StreamWriter
    ) -> None:
        try:
            await self.handler_internal(reader, writer)
        except EOFError:
            pass
        except Exception as e:
            logging.error(f"error handling connection: {e}")
        finally:
            logging.info(f"client disconnected {self.writer_info(writer)}")
            await self.close(writer)

    async def serve(self) -> None:
        self.server = await asyncio.start_server(self.handler, port=self.port)

        logging.info("starting hcproxy")

        addrs = ", ".join(str(sock.getsockname()) for sock in self.server.sockets)
        logging.info(f"serving on {addrs}")

        async with self.server:
            await self.server.serve_forever()


def main() -> None:
    logging.basicConfig(format="%(levelname)s - %(message)s", level=logging.INFO)

    port = int(os.getenv("HCPROXY_PORT", 81))
    proxy = HCProxy(port=port)
    asyncio.run(proxy.serve())


if __name__ == "__main__":
    main()
