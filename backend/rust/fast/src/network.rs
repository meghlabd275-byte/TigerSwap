//! Epoll-based network loop for the TigerSwap engine.
//!
//! The server owns its connections, tracks them by file descriptor and uses
//! `epoll` for readiness notification. Buffers are bounded so a slow or hostile
//! peer cannot exhaust memory.

use std::collections::HashMap;
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::os::unix::io::{AsRawFd, RawFd};
use std::sync::atomic::{AtomicBool, AtomicU64, AtomicUsize, Ordering};
use std::time::{Duration, Instant};

use parking_lot::RwLock;

/// Maximum bytes buffered per connection before the peer is disconnected.
pub const MAX_CONNECTION_BUFFER: usize = 4 * 1024 * 1024;

/// Maximum events returned by a single `epoll_wait` call.
const MAX_EVENTS: usize = 1024;

/// Statistics for the server.
#[derive(Debug, Clone, Copy, Default)]
pub struct ServerStats {
    pub total_connections: u64,
    pub total_requests: u64,
    pub total_bytes_in: u64,
    pub total_bytes_out: u64,
    pub active_connections: u64,
    pub dropped_connections: u64,
}

struct Connection {
    stream: TcpStream,
    id: usize,
    read_buffer: Vec<u8>,
    write_buffer: Vec<u8>,
    last_active: Instant,
}

impl Connection {
    /// Monotonic identifier assigned when the connection was accepted.
    pub fn id(&self) -> usize {
        self.id
    }
}

/// Epoll-driven TCP server.
pub struct EpollServer {
    epoll_fd: RawFd,
    listener: TcpListener,
    connections: RwLock<HashMap<RawFd, Connection>>,
    next_id: AtomicUsize,
    running: AtomicBool,

    total_connections: AtomicU64,
    total_requests: AtomicU64,
    total_bytes_in: AtomicU64,
    total_bytes_out: AtomicU64,
    dropped_connections: AtomicU64,
}

impl EpollServer {
    /// Binds the server to `addr` and prepares the epoll instance.
    pub fn new(addr: &str) -> std::io::Result<Self> {
        let listener = TcpListener::bind(addr)?;
        listener.set_nonblocking(true)?;

        let epoll_fd = unsafe { libc::epoll_create1(libc::EPOLL_CLOEXEC) };
        if epoll_fd < 0 {
            return Err(std::io::Error::last_os_error());
        }

        let mut event = libc::epoll_event {
            events: (libc::EPOLLIN) as u32,
            u64: listener.as_raw_fd() as u64,
        };

        let result = unsafe {
            libc::epoll_ctl(
                epoll_fd,
                libc::EPOLL_CTL_ADD,
                listener.as_raw_fd(),
                &mut event,
            )
        };
        if result < 0 {
            let error = std::io::Error::last_os_error();
            unsafe { libc::close(epoll_fd) };
            return Err(error);
        }

        Ok(Self {
            epoll_fd,
            listener,
            connections: RwLock::new(HashMap::new()),
            next_id: AtomicUsize::new(1),
            running: AtomicBool::new(false),
            total_connections: AtomicU64::new(0),
            total_requests: AtomicU64::new(0),
            total_bytes_in: AtomicU64::new(0),
            total_bytes_out: AtomicU64::new(0),
            dropped_connections: AtomicU64::new(0),
        })
    }

    /// Local address the server is bound to.
    pub fn local_addr(&self) -> std::io::Result<std::net::SocketAddr> {
        self.listener.local_addr()
    }

    /// Runs one poll iteration, returning the number of events handled.
    pub fn poll_once(&self, timeout_ms: i32) -> std::io::Result<usize> {
        let mut events: [libc::epoll_event; MAX_EVENTS] = unsafe { std::mem::zeroed() };

        let ready = unsafe {
            libc::epoll_wait(
                self.epoll_fd,
                events.as_mut_ptr(),
                MAX_EVENTS as i32,
                timeout_ms,
            )
        };

        if ready < 0 {
            let error = std::io::Error::last_os_error();
            if error.kind() == std::io::ErrorKind::Interrupted {
                return Ok(0);
            }
            return Err(error);
        }

        for event in events.iter().take(ready as usize) {
            let fd = event.u64 as RawFd;

            if fd == self.listener.as_raw_fd() {
                self.accept_pending();
            } else if event.events & (libc::EPOLLIN as u32) != 0 {
                self.handle_read(fd);
            } else if event.events & (libc::EPOLLHUP as u32) != 0
                || event.events & (libc::EPOLLERR as u32) != 0
            {
                self.close_connection(fd);
            }
        }

        Ok(ready as usize)
    }

    /// Runs the loop until `stop` is called.
    pub fn start(&self) {
        self.running.store(true, Ordering::SeqCst);

        while self.running.load(Ordering::SeqCst) {
            if let Err(_error) = self.poll_once(100) {
                // A transient poll error must not tear the server down.
                std::thread::sleep(Duration::from_millis(1));
            }
        }
    }

    pub fn stop(&self) {
        self.running.store(false, Ordering::SeqCst);
    }

    fn accept_pending(&self) {
        loop {
            match self.listener.accept() {
                Ok((stream, _peer)) => {
                    if self.register(stream).is_err() {
                        self.dropped_connections.fetch_add(1, Ordering::Relaxed);
                    }
                }
                Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => break,
                Err(_) => break,
            }
        }
    }

    /// Registers an accepted stream with the epoll set.
    pub fn register(&self, stream: TcpStream) -> std::io::Result<()> {
        stream.set_nonblocking(true)?;
        let fd = stream.as_raw_fd();

        let mut event = libc::epoll_event {
            events: (libc::EPOLLIN | libc::EPOLLET | libc::EPOLLRDHUP) as u32,
            u64: fd as u64,
        };

        let result = unsafe { libc::epoll_ctl(self.epoll_fd, libc::EPOLL_CTL_ADD, fd, &mut event) };
        if result < 0 {
            return Err(std::io::Error::last_os_error());
        }

        let id = self.next_id.fetch_add(1, Ordering::SeqCst);
        self.connections.write().insert(
            fd,
            Connection {
                stream,
                id,
                read_buffer: Vec::with_capacity(8192),
                write_buffer: Vec::new(),
                last_active: Instant::now(),
            },
        );
        self.total_connections.fetch_add(1, Ordering::Relaxed);
        Ok(())
    }

    fn handle_read(&self, fd: RawFd) {
        let mut scratch = [0u8; 8192];

        loop {
            let bytes_read = {
                let mut connections = self.connections.write();
                let connection = match connections.get_mut(&fd) {
                    Some(connection) => connection,
                    None => return,
                };

                match connection.stream.read(&mut scratch) {
                    Ok(0) => {
                        drop(connections);
                        self.close_connection(fd);
                        return;
                    }
                    Ok(count) => {
                        connection.read_buffer.extend_from_slice(&scratch[..count]);
                        connection.last_active = Instant::now();
                        count
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => return,
                    Err(error) if error.kind() == std::io::ErrorKind::Interrupted => continue,
                    Err(_) => {
                        drop(connections);
                        self.close_connection(fd);
                        return;
                    }
                }
            };

            self.total_bytes_in
                .fetch_add(bytes_read as u64, Ordering::Relaxed);
            self.total_requests.fetch_add(1, Ordering::Relaxed);

            let overflowed = {
                let connections = self.connections.read();
                connections
                    .get(&fd)
                    .map(|connection| connection.read_buffer.len() > MAX_CONNECTION_BUFFER)
                    .unwrap_or(false)
            };

            if overflowed {
                self.dropped_connections.fetch_add(1, Ordering::Relaxed);
                self.close_connection(fd);
                return;
            }
        }
    }

    /// Queues bytes for a connection and flushes what the socket accepts.
    pub fn send(&self, fd: RawFd, data: &[u8]) -> std::io::Result<usize> {
        let mut connections = self.connections.write();
        let connection = match connections.get_mut(&fd) {
            Some(connection) => connection,
            None => {
                return Err(std::io::Error::new(
                    std::io::ErrorKind::NotFound,
                    "connection not registered",
                ))
            }
        };

        connection.write_buffer.extend_from_slice(data);
        let flushed = flush(&mut connection.stream, &mut connection.write_buffer)?;
        self.total_bytes_out
            .fetch_add(flushed as u64, Ordering::Relaxed);
        Ok(flushed)
    }

    /// Takes the buffered inbound bytes for a connection.
    pub fn take_read_buffer(&self, fd: RawFd) -> Option<Vec<u8>> {
        let mut connections = self.connections.write();
        connections
            .get_mut(&fd)
            .map(|connection| std::mem::take(&mut connection.read_buffer))
    }

    pub fn connection_count(&self) -> usize {
        self.connections.read().len()
    }

    /// Identifier of a registered connection, if it is still open.
    pub fn connection_id(&self, fd: RawFd) -> Option<usize> {
        self.connections.read().get(&fd).map(Connection::id)
    }

    pub fn is_running(&self) -> bool {
        self.running.load(Ordering::SeqCst)
    }

    /// Closes a connection and removes it from the epoll set.
    pub fn close_connection(&self, fd: RawFd) {
        let mut connections = self.connections.write();
        if connections.remove(&fd).is_some() {
            unsafe {
                libc::epoll_ctl(self.epoll_fd, libc::EPOLL_CTL_DEL, fd, std::ptr::null_mut());
            }
            // The `TcpStream` in the removed `Connection` owns `fd` and closes
            // it on drop. Calling `close` here as well would release the
            // descriptor a second time, and if another thread had meanwhile
            // been assigned that number the wrong socket would be torn down.
        }
    }

    pub fn get_stats(&self) -> ServerStats {
        ServerStats {
            total_connections: self.total_connections.load(Ordering::Relaxed),
            total_requests: self.total_requests.load(Ordering::Relaxed),
            total_bytes_in: self.total_bytes_in.load(Ordering::Relaxed),
            total_bytes_out: self.total_bytes_out.load(Ordering::Relaxed),
            active_connections: self.connections.read().len() as u64,
            dropped_connections: self.dropped_connections.load(Ordering::Relaxed),
        }
    }
}

impl Drop for EpollServer {
    fn drop(&mut self) {
        self.stop();
        let descriptors: Vec<RawFd> = self.connections.read().keys().copied().collect();
        for fd in descriptors {
            self.close_connection(fd);
        }
        unsafe { libc::close(self.epoll_fd) };
    }
}

fn flush(stream: &mut TcpStream, buffer: &mut Vec<u8>) -> std::io::Result<usize> {
    if buffer.is_empty() {
        return Ok(0);
    }

    match stream.write(buffer) {
        Ok(written) => {
            buffer.drain(..written);
            Ok(written)
        }
        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => Ok(0),
        Err(error) if error.kind() == std::io::ErrorKind::Interrupted => Ok(0),
        Err(error) => Err(error),
    }
}

/// Circular buffer with explicit read and write cursors.
///
/// The number of live bytes is tracked explicitly so that a full buffer is
/// distinguishable from an empty one when the cursors coincide.
pub struct ZeroCopyBuffer {
    data: Vec<u8>,
    read_pos: usize,
    write_pos: usize,
    len: usize,
    capacity: usize,
}

impl ZeroCopyBuffer {
    pub fn new(capacity: usize) -> Self {
        let capacity = capacity.max(1);
        Self {
            data: vec![0; capacity],
            read_pos: 0,
            write_pos: 0,
            len: 0,
            capacity,
        }
    }

    /// Writes as much of `data` as fits, returning the number of bytes written.
    pub fn write(&mut self, data: &[u8]) -> usize {
        let to_write = self.free_space().min(data.len());

        for (index, byte) in data.iter().take(to_write).enumerate() {
            let position = (self.write_pos + index) % self.capacity;
            self.data[position] = *byte;
        }

        self.write_pos = (self.write_pos + to_write) % self.capacity;
        self.len += to_write;
        to_write
    }

    /// Reads up to `len` bytes, returning a copy of what was read.
    pub fn read(&mut self, len: usize) -> Option<Vec<u8>> {
        if self.len == 0 {
            return None;
        }

        let to_read = self.len.min(len);
        let mut out = Vec::with_capacity(to_read);
        for index in 0..to_read {
            out.push(self.data[(self.read_pos + index) % self.capacity]);
        }

        self.read_pos = (self.read_pos + to_read) % self.capacity;
        self.len -= to_read;
        Some(out)
    }

    pub fn available(&self) -> usize {
        self.len
    }

    pub fn free_space(&self) -> usize {
        self.capacity - self.len
    }

    pub fn is_empty(&self) -> bool {
        self.len == 0
    }

    pub fn capacity(&self) -> usize {
        self.capacity
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::net::TcpStream;

    #[test]
    fn buffer_round_trip() {
        let mut buffer = ZeroCopyBuffer::new(1024);
        assert_eq!(buffer.write(b"Hello, World!"), 13);
        assert_eq!(buffer.available(), 13);
        assert_eq!(buffer.read(5).unwrap(), b"Hello".to_vec());
        assert_eq!(buffer.available(), 8);
    }

    #[test]
    fn buffer_wraps_around() {
        let mut buffer = ZeroCopyBuffer::new(10);
        assert_eq!(buffer.write(b"ABCDEFGHIJ"), 10);
        assert_eq!(buffer.read(5).unwrap(), b"ABCDE".to_vec());
        assert_eq!(buffer.write(b"XYZ"), 3);
        assert_eq!(buffer.read(8).unwrap(), b"FGHIJXYZ".to_vec());
        assert!(buffer.is_empty());
    }

    #[test]
    fn buffer_refuses_overflow() {
        let mut buffer = ZeroCopyBuffer::new(4);
        assert_eq!(buffer.write(b"abcdef"), 4);
        assert_eq!(buffer.free_space(), 0);
        assert_eq!(buffer.write(b"x"), 0);
    }

    #[test]
    fn buffer_read_returns_none_when_empty() {
        let mut buffer = ZeroCopyBuffer::new(8);
        assert!(buffer.read(4).is_none());
    }

    #[test]
    fn server_accepts_and_reads_a_connection() {
        let server = EpollServer::new("127.0.0.1:0").unwrap();
        let addr = server.local_addr().unwrap();

        let mut client = TcpStream::connect(addr).unwrap();
        client.write_all(b"ping-from-client").unwrap();
        client.flush().unwrap();

        // Poll until the connection is registered and the payload arrives.
        let mut received = Vec::new();
        for _ in 0..100 {
            server.poll_once(20).unwrap();

            let fds: Vec<RawFd> = server.connections.read().keys().copied().collect();
            for fd in fds {
                if let Some(bytes) = server.take_read_buffer(fd) {
                    received.extend_from_slice(&bytes);
                }
            }

            if received.len() >= b"ping-from-client".len() {
                break;
            }
        }

        assert_eq!(received, b"ping-from-client".to_vec());
        let stats = server.get_stats();
        assert_eq!(stats.total_connections, 1);
        assert!(stats.total_bytes_in >= 16);
    }

    #[test]
    fn server_reports_dropped_connections_on_close() {
        let server = EpollServer::new("127.0.0.1:0").unwrap();
        let addr = server.local_addr().unwrap();

        let client = TcpStream::connect(addr).unwrap();
        for _ in 0..50 {
            server.poll_once(10).unwrap();
            if server.connection_count() > 0 {
                break;
            }
        }
        assert_eq!(server.connection_count(), 1);

        drop(client);
        for _ in 0..50 {
            server.poll_once(10).unwrap();
            if server.connection_count() == 0 {
                break;
            }
        }
        assert_eq!(server.connection_count(), 0);
    }
}
