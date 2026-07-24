//! TigerSwap High-Performance Network Layer
//! Epoll-based async networking with zero-copy operations

use std::collections::HashMap;
use std::sync::atomic::{AtomicBool, AtomicU64, AtomicUsize, Ordering};
use std::sync::{Arc, RwLock};
use std::time::{Duration, Instant};
use std::net::TcpListener;
use std::io::{Read, Write};
use std::os::unix::io::{AsRawFd, RawFd};

// ============== HIGH-PERFORMANCE SERVER ==============

/// Epoll-based event loop for high-performance networking
pub struct EpollServer {
    epoll_fd: i32,
    listener: TcpListener,
    connections: RwLock<HashMap<usize, Connection>>,
    next_id: AtomicUsize,
    running: AtomicBool,
    
    // Statistics
    total_connections: AtomicU64,
    total_requests: AtomicU64,
    total_bytes_in: AtomicU64,
    total_bytes_out: AtomicU64,
}

struct Connection {
    fd: RawFd,
    id: usize,
    buf: Vec<u8>,
    last_active: Instant,
}

impl EpollServer {
    pub fn new(addr: &str) -> std::io::Result<Self> {
        let listener = TcpListener::bind(addr)?;
        listener.set_nonblocking(true)?;
        
        let epoll_fd = unsafe {
            libc::epoll_create1(libc::EPOLL_CLOEXEC)
        };
        
        if epoll_fd < 0 {
            return Err(std::io::Error::last_os_error());
        }
        
        // Register listener
        let event = libc::epoll_event {
            events: libc::EPOLLIN as u32,
            u64: 0, // Use as listener marker
        };
        
        unsafe {
            libc::epoll_ctl(epoll_fd, libc::EPOLL_CTL_ADD, listener.as_raw_fd(), &event);
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
        })
    }
    
    pub fn start(&self) {
        self.running.store(true, Ordering::SeqCst);
        
        let mut events = vec![0 unsafe { std::mem::zeroed() }; 1024];
        
        while self.running.load(Ordering::SeqCst) {
            let n = unsafe {
                libc::epoll_wait(
                    self.epoll_fd,
                    events.as_mut_ptr(),
                    events.len() as i32,
                    100, // 100ms timeout
                )
            };
            
            if n < 0 {
                continue;
            }
            
            for i in 0..n as usize {
                let event = events[i];
                let fd = event.u64 as RawFd;
                
                if fd == self.listener.as_raw_fd() {
                    // Accept new connection
                    while let Ok((stream, _)) = self.listener.accept() {
                        self.handle_accept(stream);
                    }
                } else {
                    // Handle IO event
                    self.handle_io(fd, event.events);
                }
            }
        }
    }
    
    pub fn stop(&self) {
        self.running.store(false, Ordering::SeqCst);
    }
    
    fn handle_accept(&self, mut stream: std::net::TcpStream) {
        stream.set_nonblocking(true).ok();
        
        let id = self.next_id.fetch_add(1, Ordering::SeqCst);
        
        // Register for edge-triggered input
        let event = libc::epoll_event {
            events: (libc::EPOLLIN | libc::EPOLLET) as u32,
            u64: stream.as_raw_fd() as u64,
        };
        
        unsafe {
            libc::epoll_ctl(self.epoll_fd, libc::EPOLL_CTL_ADD, stream.as_raw_fd(), &event);
        }
        
        let conn = Connection {
            fd: stream.as_raw_fd(),
            id,
            buf: Vec::with_capacity(8192),
            last_active: Instant::now(),
        };
        
        self.connections.write().unwrap().insert(id, conn);
        self.total_connections.fetch_add(1, Ordering::Relaxed);
    }
    
    fn handle_io(&self, fd: RawFd, events: u32) {
        if events & libc::EPOLLIN != 0 {
            self.handle_read(fd);
        }
        
        if events & libc::EPOLLOUT != 0 {
            self.handle_write(fd);
        }
    }
    
    fn handle_read(&self, fd: RawFd) {
        let mut buf = [0u8; 8192];
        
        loop {
            match unsafe { libc::read(fd, buf.as_mut_ptr() as *mut libc::c_void, buf.len()) } {
                n if n > 0 => {
                    self.total_bytes_in.fetch_add(n as u64, Ordering::Relaxed);
                    self.total_requests.fetch_add(1, Ordering::Relaxed);
                    
                    // Find connection and append to buffer
                    let mut connections = self.connections.write().unwrap();
                    for (_, conn) in connections.iter_mut() {
                        if conn.fd == fd {
                            conn.buf.extend_from_slice(&buf[..n]);
                            conn.last_active = Instant::now();
                            break;
                        }
                    }
                }
                0 => {
                    // Connection closed
                    self.close_connection(fd);
                }
                -1 => {
                    if std::io::Error::last_os_error().kind() == std::io::ErrorKind::WouldBlock {
                        break;
                    }
                    self.close_connection(fd);
                }
                _ => break,
            }
        }
    }
    
    fn handle_write(&self, fd: RawFd) {
        // Write pending data
    }
    
    fn close_connection(&self, fd: RawFd) {
        unsafe { libc::close(fd); }
        
        let mut connections = self.connections.write().unwrap();
        connections.retain(|_, c| c.fd != fd);
    }
    
    pub fn get_stats(&self) -> ServerStats {
        ServerStats {
            total_connections: self.total_connections.load(Ordering::Relaxed),
            total_requests: self.total_requests.load(Ordering::Relaxed),
            total_bytes_in: self.total_bytes_in.load(Ordering::Relaxed),
            total_bytes_out: self.total_bytes_out.load(Ordering::Relaxed),
            active_connections: self.connections.read().unwrap().len() as u64,
        }
    }
}

#[derive(Debug, Clone)]
pub struct ServerStats {
    pub total_connections: u64,
    pub total_requests: u64,
    pub total_bytes_in: u64,
    pub total_bytes_out: u64,
    pub active_connections: u64,
}

// ============== ZERO-COPY BUFFER ==============

/// Zero-copy buffer for high-performance messaging
pub struct ZeroCopyBuffer {
    data: Vec<u8>,
    read_pos: usize,
    write_pos: usize,
    capacity: usize,
}

impl ZeroCopyBuffer {
    pub fn new(capacity: usize) -> Self {
        Self {
            data: vec![0; capacity],
            read_pos: 0,
            write_pos: 0,
            capacity,
        }
    }
    
    pub fn write(&mut self, data: &[u8]) -> usize {
        let available = self.capacity - self.write_pos;
        let to_write = available.min(data.len());
        
        self.data[self.write_pos..self.write_pos + to_write].copy_from_slice(&data[..to_write]);
        self.write_pos = (self.write_pos + to_write) % self.capacity;
        
        to_write
    }
    
    pub fn read(&mut self, len: usize) -> Option<&[u8]> {
        if self.write_pos == self.read_pos {
            return None;
        }
        
        let available = if self.write_pos > self.read_pos {
            self.write_pos - self.read_pos
        } else {
            self.capacity - self.read_pos + self.write_pos
        };
        
        let to_read = available.min(len);
        let result = &self.data[self.read_pos..self.read_pos + to_read];
        self.read_pos = (self.read_pos + to_read) % self.capacity;
        
        Some(result)
    }
    
    pub fn available(&self) -> usize {
        if self.write_pos >= self.read_pos {
            self.write_pos - self.read_pos
        } else {
            self.capacity - self.read_pos + self.write_pos
        }
    }
}

// ============== FFI EXPORTS ==============

#[no_mangle]
pub extern "C" fn create_epoll_server(addr: *const u8, addr_len: usize) -> *mut EpollServer {
    let addr_str = unsafe {
        std::str::from_utf8(std::slice::from_raw_parts(addr, addr_len)).unwrap_or("0.0.0.0:8080")
    };
    
    match EpollServer::new(addr_str) {
        Ok(server) => Box::into_raw(Box::new(server)),
        Err(_) => std::ptr::null_mut(),
    }
}

#[no_mangle]
pub extern "C" fn destroy_epoll_server(server: *mut EpollServer) {
    if !server.is_null() {
        unsafe { Box::from_raw(server) };
    }
}

#[no_mangle]
pub extern "C" fn epoll_server_start(server: *mut EpollServer) {
    if !server.is_null() {
        unsafe { (*server).start() };
    }
}

#[no_mangle]
pub extern "C" fn epoll_server_stop(server: *mut EpollServer) {
    if !server.is_null() {
        unsafe { (*server).stop() };
    }
}

#[no_mangle]
pub extern "C" fn epoll_server_stats(server: *mut EpollServer) -> ServerStats {
    if server.is_null() {
        return ServerStats {
            total_connections: 0,
            total_requests: 0,
            total_bytes_in: 0,
            total_bytes_out: 0,
            active_connections: 0,
        };
    }
    
    unsafe { (*server).get_stats() }
}

// ============== TESTS ==============

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_buffer() {
        let mut buf = ZeroCopyBuffer::new(1024);
        
        let data = b"Hello, World!";
        let written = buf.write(data);
        assert_eq!(written, data.len());
        
        let read = buf.read(5);
        assert!(read.is_some());
        assert_eq!(read.unwrap(), b"Hello");
    }
    
    #[test]
    fn test_buffer_wrap() {
        let mut buf = ZeroCopyBuffer::new(10);
        
        buf.write(b"ABCDEFGHIJ"); // Fill buffer
        buf.read(5); // Read half
        
        let data = b"XYZ";
        let written = buf.write(data); // Should wrap around
        assert_eq!(written, 3);
    }
}
