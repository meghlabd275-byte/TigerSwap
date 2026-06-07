/**
 * TigerSwap Infrastructure - Monitoring System
 * 
 * Monitoring, alerting, and observability for TigerSwap ecosystem.
 * Zero external dependencies - fully native implementation.
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

// ============================================================================
// Types & Interfaces
// ============================================================================

export interface Metric {
  name: string;
  value: number;
  timestamp: number;
  labels: Record<string, string>;
}

export interface Alert {
  id: string;
  name: string;
  severity: 'critical' | 'warning' | 'info';
  message: string;
  timestamp: number;
  resolved: boolean;
}

export interface HealthCheck {
  service: string;
  status: 'healthy' | 'degraded' | 'down';
  latency: number;
  lastCheck: number;
}

export interface LogEntry {
  timestamp: number;
  level: 'debug' | 'info' | 'warn' | 'error';
  message: string;
  metadata?: Record<string, any>;
}

// ============================================================================
// Metrics Collector
// ============================================================================

export class MetricsCollector {
  private metrics: Map<string, Metric[]>;
  private collectors: Map<string, MetricCollector>;

  constructor() {
    this.metrics = new Map();
    this.collectors = new Map();
  }

  /**
   * Record metric
   */
  record(name: string, value: number, labels: Record<string, string> = {}): void {
    const metric: Metric = {
      name,
      value,
      timestamp: Date.now(),
      labels,
    };

    const existing = this.metrics.get(name) || [];
    existing.push(metric);
    this.metrics.set(name, existing);

    // Keep only last 1000 metrics per name
    if (existing.length > 1000) {
      existing.shift();
    }
  }

  /**
   * Register collector
   */
  registerCollector(name: string, collector: MetricCollector): void {
    this.collectors.set(name, collector);
  }

  /**
   * Get metrics
   */
  getMetrics(name: string, since?: number): Metric[] {
    const metrics = this.metrics.get(name) || [];
    if (since) {
      return metrics.filter(m => m.timestamp >= since);
    }
    return metrics;
  }

  /**
   * Get aggregated metrics
   */
  aggregate(name: string, window: number): {
    avg: number;
    min: number;
    max: number;
    count: number;
  } {
    const since = Date.now() - window;
    const metrics = this.getMetrics(name, since);

    if (metrics.length === 0) {
      return { avg: 0, min: 0, max: 0, count: 0 };
    }

    const values = metrics.map(m => m.value);
    return {
      avg: values.reduce((a, b) => a + b, 0) / values.length,
      min: Math.min(...values),
      max: Math.max(...values),
      count: values.length,
    };
  }
}

interface MetricCollector {
  collect(): Promise<Metric[]>;
}

// ============================================================================
// Health Checker
// ============================================================================

export class HealthChecker {
  private services: Map<string, HealthCheck>;
  private checkInterval: number = 30000;

  constructor() {
    this.services = new Map();
  }

  /**
   * Register service
   */
  registerService(name: string, checkFn: () => Promise<boolean>): void {
    this.services.set(name, {
      service: name,
      status: 'healthy',
      latency: 0,
      lastCheck: Date.now(),
    });

    // Start periodic checks
    setInterval(async () => {
      await this.checkService(name, checkFn);
    }, this.checkInterval);
  }

  /**
   * Check service
   */
  async checkService(name: string, checkFn: () => Promise<boolean>): Promise<HealthCheck> {
    const start = Date.now();
    try {
      const healthy = await checkFn();
      const service: HealthCheck = {
        service: name,
        status: healthy ? 'healthy' : 'down',
        latency: Date.now() - start,
        lastCheck: Date.now(),
      };
      this.services.set(name, service);
      return service;
    } catch (error) {
      const service: HealthCheck = {
        service: name,
        status: 'down',
        latency: Date.now() - start,
        lastCheck: Date.now(),
      };
      this.services.set(name, service);
      return service;
    }
  }

  /**
   * Get all health checks
   */
  getHealthChecks(): HealthCheck[] {
    return Array.from(this.services.values());
  }

  /**
   * Get overall health
   */
  getOverallHealth(): 'healthy' | 'degraded' | 'down' {
    const checks = this.getHealthChecks();
    if (checks.every(c => c.status === 'healthy')) return 'healthy';
    if (checks.some(c => c.status === 'down')) return 'down';
    return 'degraded';
  }
}

// ============================================================================
// Alert Manager
// ============================================================================

export class AlertManager {
  private alerts: Map<string, Alert>;
  private handlers: AlertHandler[];

  constructor() {
    this.alerts = new Map();
    this.handlers = [];
  }

  /**
   * Register alert handler
   */
  registerHandler(handler: AlertHandler): void {
    this.handlers.push(handler);
  }

  /**
   * Create alert
   */
  createAlert(name: string, severity: Alert['severity'], message: string): Alert {
    const alert: Alert = {
      id: `alert_${Date.now()}_${Math.random().toString(36).slice(2, 9)}`,
      name,
      severity,
      message,
      timestamp: Date.now(),
      resolved: false,
    };

    this.alerts.set(alert.id, alert);

    // Notify handlers
    this.handlers.forEach(h => h.handle(alert));

    return alert;
  }

  /**
   * Resolve alert
   */
  resolveAlert(alertId: string): void {
    const alert = this.alerts.get(alertId);
    if (alert) {
      alert.resolved = true;
    }
  }

  /**
   * Get active alerts
   */
  getActiveAlerts(): Alert[] {
    return Array.from(this.alerts.values()).filter(a => !a.resolved);
  }

  /**
   * Get alerts by severity
   */
  getAlertsBySeverity(severity: Alert['severity']): Alert[] {
    return this.getActiveAlerts().filter(a => a.severity === severity);
  }
}

interface AlertHandler {
  handle(alert: Alert): void;
}

// ============================================================================
// Logger
// ============================================================================

export class Logger {
  private logs: LogEntry[];
  private maxLogs: number = 10000;

  constructor() {
    this.logs = [];
  }

  /**
   * Log debug
   */
  debug(message: string, metadata?: Record<string, any>): void {
    this.log('debug', message, metadata);
  }

  /**
   * Log info
   */
  info(message: string, metadata?: Record<string, any>): void {
    this.log('info', message, metadata);
  }

  /**
   * Log warning
   */
  warn(message: string, metadata?: Record<string, any>): void {
    this.log('warn', message, metadata);
  }

  /**
   * Log error
   */
  error(message: string, metadata?: Record<string, any>): void {
    this.log('error', message, metadata);
  }

  /**
   * Get logs
   */
  getLogs(level?: LogEntry['level'], since?: number): LogEntry[] {
    let logs = this.logs;
    if (level) {
      logs = logs.filter(l => l.level === level);
    }
    if (since) {
      logs = logs.filter(l => l.timestamp >= since);
    }
    return logs;
  }

  /**
   * Get error logs
   */
  getErrors(since?: number): LogEntry[] {
    return this.getLogs('error', since);
  }

  private log(level: LogEntry['level'], message: string, metadata?: Record<string, any>): void {
    const entry: LogEntry = {
      timestamp: Date.now(),
      level,
      message,
      metadata,
    };
    this.logs.push(entry);
    if (this.logs.length > this.maxLogs) {
      this.logs.shift();
    }
  }
}

// ============================================================================
// Prometheus Exporter
// ============================================================================

export class PrometheusExporter {
  private metrics: Map<string, string>;

  constructor() {
    this.metrics = new Map();
  }

  /**
   * Add gauge
   */
  addGauge(name: string, value: number, labels: Record<string, string> = {}): void {
    const labelStr = Object.entries(labels).map(([k, v]) => `${k}="${v}"`).join(',');
    const metricName = labelStr ? `${name}{${labelStr}}` : name;
    this.metrics.set(metricName, value.toString());
  }

  /**
   * Add counter
   */
  addCounter(name: string, value: number, labels: Record<string, string> = {}): void {
    this.addGauge(name, value, labels);
  }

  /**
   * Add histogram
   */
  addHistogram(name: string, value: number, labels: Record<string, string> = {}): void {
    this.addGauge(`${name}_bucket`, value, { ...labels, le: '+Inf' });
  }

  /**
   * Export metrics
   */
  export(): string {
    let output = '';
    for (const [name, value] of this.metrics) {
      output += `${name} ${value}\n`;
    }
    return output;
  }
}

// ============================================================================
// Dashboard Generator
// ============================================================================

export class DashboardGenerator {
  /**
   * Generate dashboard JSON
   */
  generateDashboard(title: string, panels: DashboardPanel[]): Dashboard {
    return {
      title,
      panels,
      version: '1.0.0',
      schemaVersion: 30,
    };
  }

  /**
   * Generate panel
   */
  generatePanel(title: string, type: 'graph' | 'stat' | 'table', metrics: string[]): DashboardPanel {
    return {
      title,
      type,
      targets: metrics.map(m => ({ expr: m })),
    };
  }
}

interface Dashboard {
  title: string;
  panels: DashboardPanel[];
  version: string;
  schemaVersion: number;
}

interface DashboardPanel {
  title: string;
  type: string;
  targets: { expr: string }[];
}

// ============================================================================
// Export
// ============================================================================

export default {
  MetricsCollector,
  HealthChecker,
  AlertManager,
  Logger,
  PrometheusExporter,
  DashboardGenerator,
};