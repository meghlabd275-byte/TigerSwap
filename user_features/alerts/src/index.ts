/**
 * TigerSwap User Features - Alerts Module
 * 
 * Native price alerts and notifications.
 * Zero external dependencies - fully native implementation.
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

export interface Alert {
  id: string;
  user: string;
  type: 'price' | 'percent' | 'volume' | 'whale';
  token: string;
  condition: 'above' | 'below' | 'crosses';
  value: number;
  triggered: boolean;
  triggeredAt?: number;
}

export interface Notification {
  id: string;
  user: string;
  type: 'alert' | 'info' | 'warning' | 'error';
  title: string;
  message: string;
  timestamp: number;
  read: boolean;
}

export interface WhaleAlert {
  address: string;
  token: string;
  amount: bigint;
  value: number;
  direction: 'in' | 'out';
  timestamp: number;
}

export class AlertSystem {
  private alerts: Map<string, Alert[]>;
  private notifications: Map<string, Notification[]>;
  private whaleAlerts: Map<string, WhaleAlert[]>;
  private priceFeeds: Map<string, number>;

  constructor() {
    this.alerts = new Map();
    this.notifications = new Map();
    this.whaleAlerts = new Map();
    this.priceFeeds = new Map();
  }

  /**
   * Create alert
   */
  createAlert(user: string, type: Alert['type'], token: string, condition: Alert['condition'], value: number): Alert {
    const alert: Alert = {
      id: `alert_${Date.now()}_${Math.random().toString(36).slice(2, 9)}`,
      user,
      type,
      token,
      condition,
      value,
      triggered: false,
    };

    const userAlerts = this.alerts.get(user) || [];
    userAlerts.push(alert);
    this.alerts.set(user, userAlerts);

    return alert;
  }

  /**
   * Update price feed
   */
  updatePrice(token: string, price: number): void {
    const oldPrice = this.priceFeeds.get(token) || price;
    this.priceFeeds.set(token, price);

    // Check alerts
    for (const [user, alerts] of this.alerts) {
      for (const alert of alerts) {
        if (alert.token === token && !alert.triggered) {
          let triggered = false;

          if (alert.condition === 'above' && price > alert.value) {
            triggered = true;
          } else if (alert.condition === 'below' && price < alert.value) {
            triggered = true;
          } else if (alert.condition === 'crosses') {
            triggered = (oldPrice < alert.value && price > alert.value) ||
                       (oldPrice > alert.value && price < alert.value);
          }

          if (triggered) {
            alert.triggered = true;
            alert.triggeredAt = Date.now();
            this.sendNotification(user, 'alert', 'Price Alert', `${token} crossed ${alert.value}`);
          }
        }
      }
    }
  }

  /**
   * Track whale
   */
  trackWhale(address: string, token: string, amount: bigint, direction: 'in' | 'out'): void {
    const value = Number(amount) * (this.priceFeeds.get(token) || 0);
    
    if (value > 100000) { // $100k threshold
      const whaleAlert: WhaleAlert = {
        address,
        token,
        amount,
        value,
        direction,
        timestamp: Date.now(),
      };

      const alerts = this.whaleAlerts.get(token) || [];
      alerts.push(whaleAlert);
      this.whaleAlerts.set(token, alerts);

      // Notify users
      for (const [user, userAlerts] of this.alerts) {
        const priceAlerts = userAlerts.filter(a => a.type === 'whale' && a.token === token);
        if (priceAlerts.length > 0) {
          this.sendNotification(user, 'warning', 'Whale Alert', 
            `Large ${direction === 'in' ? 'deposit' : 'withdrawal'} of ${amount} ${token}`);
        }
      }
    }
  }

  /**
   * Send notification
   */
  sendNotification(user: string, type: Notification['type'], title: string, message: string): void {
    const notification: Notification = {
      id: `notif_${Date.now()}`,
      user,
      type,
      title,
      message,
      timestamp: Date.now(),
      read: false,
    };

    const userNotifs = this.notifications.get(user) || [];
    userNotifs.push(notification);
    this.notifications.set(user, userNotifs);
  }

  /**
   * Get alerts
   */
  getAlerts(user: string, triggered?: boolean): Alert[] {
    const alerts = this.alerts.get(user) || [];
    if (triggered !== undefined) {
      return alerts.filter(a => a.triggered === triggered);
    }
    return alerts;
  }

  /**
   * Get notifications
   */
  getNotifications(user: string, unreadOnly?: boolean): Notification[] {
    const notifs = this.notifications.get(user) || [];
    if (unreadOnly) {
      return notifs.filter(n => !n.read);
    }
    return notifs;
  }

  /**
   * Mark as read
   */
  markAsRead(user: string, notificationId: string): void {
    const notifs = this.notifications.get(user);
    if (notifs) {
      const notif = notifs.find(n => n.id === notificationId);
      if (notif) {
        notif.read = true;
      }
    }
  }

  /**
   * Delete alert
   */
  deleteAlert(user: string, alertId: string): void {
    const alerts = this.alerts.get(user);
    if (alerts) {
      const index = alerts.findIndex(a => a.id === alertId);
      if (index !== -1) {
        alerts.splice(index, 1);
      }
    }
  }
}

export default AlertSystem;