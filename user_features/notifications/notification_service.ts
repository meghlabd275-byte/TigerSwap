/**
 * TigerSwap Notification Service
 * Real-time notifications for price alerts, order fills, and more
 */

// ============================================================================
// Types & Interfaces
// ============================================================================

export type NotificationType = 
  | 'price_alert'
  | 'order_filled'
  | 'order_partial'
  | 'order_cancelled'
  | 'swap_success'
  | 'swap_failed'
  | 'liquidity_added'
  | 'liquidity_removed'
  | 'large_transaction'
  | 'system_update'
  | 'security_alert';

export type NotificationChannel = 'in_app' | 'email' | 'push' | 'sms' | 'discord' | 'telegram';

export interface Notification {
  id: string;
  type: NotificationType;
  title: string;
  message: string;
  data?: Record<string, any>;
  read: boolean;
  dismissed: boolean;
  createdAt: number;
  expiresAt?: number;
  channels: NotificationChannel[];
  userId?: string;
  walletAddress?: string;
}

export interface PriceAlert {
  id: string;
  tokenAddress: string;
  tokenSymbol: string;
  targetPrice: number;
  condition: 'above' | 'below' | 'crosses';
  triggered: boolean;
  triggeredAt?: number;
  createdAt: number;
  userId: string;
}

export interface NotificationPreferences {
  email: boolean;
  push: boolean;
  sms: boolean;
  discord: boolean;
  telegram: boolean;
  priceAlerts: boolean;
  orderUpdates: boolean;
  swapUpdates: boolean;
  liquidityUpdates: boolean;
  largeTransactions: boolean;
  systemUpdates: boolean;
  minTransactionThreshold: number;
}

export interface NotificationTemplate {
  id: string;
  type: NotificationType;
  subject: string;
  bodyTemplate: string;
  channels: NotificationChannel[];
}

// ============================================================================
// Default Preferences
// ============================================================================

const DEFAULT_PREFERENCES: NotificationPreferences = {
  email: true,
  push: true,
  sms: false,
  discord: false,
  telegram: false,
  priceAlerts: true,
  orderUpdates: true,
  swapUpdates: true,
  liquidityUpdates: true,
  largeTransactions: true,
  systemUpdates: true,
  minTransactionThreshold: 10000, // $10,000
};

// ============================================================================
// Notification Service
// ============================================================================

export class NotificationService {
  private notifications: Map<string, Notification> = new Map();
  private priceAlerts: Map<string, PriceAlert> = new Map();
  private preferences: Map<string, NotificationPreferences> = new Map();
  private subscribers: Map<string, ((notification: Notification) => void)[]> = new Map();
  private priceCheckInterval: NodeJS.Timeout | null = null;

  constructor() {
    this.startPriceMonitoring();
  }

  // ============================================================================
  // Notification Management
  // ============================================================================

  /**
   * Send a notification to a user
   */
  async sendNotification(
    userId: string,
    notification: Omit<Notification, 'id' | 'createdAt' | 'read' | 'dismissed'>
  ): Promise<Notification> {
    const id = this.generateId();
    const fullNotification: Notification = {
      ...notification,
      id,
      read: false,
      dismissed: false,
      createdAt: Date.now(),
    };

    this.notifications.set(id, fullNotification);

    // Get user preferences
    const prefs = this.getPreferences(userId);

    // Send to each enabled channel
    for (const channel of notification.channels) {
      if (this.isChannelEnabled(prefs, channel, notification.type)) {
        await this.sendToChannel(userId, fullNotification, channel);
      }
    }

    // Notify subscribers
    this.notifySubscribers(userId, fullNotification);

    return fullNotification;
  }

  /**
   * Get all notifications for a user
   */
  async getNotifications(
    userId: string,
    options?: {
      unreadOnly?: boolean;
      type?: NotificationType;
      limit?: number;
      offset?: number;
    }
  ): Promise<Notification[]> {
    let notifications = Array.from(this.notifications.values())
      .filter(n => n.userId === userId || n.walletAddress === userId);

    if (options?.unreadOnly) {
      notifications = notifications.filter(n => !n.read);
    }

    if (options?.type) {
      notifications = notifications.filter(n => n.type === options.type);
    }

    // Sort by creation time descending
    notifications.sort((a, b) => b.createdAt - a.createdAt);

    const offset = options?.offset || 0;
    const limit = options?.limit || 50;

    return notifications.slice(offset, offset + limit);
  }

  /**
   * Mark notification as read
   */
  async markAsRead(notificationId: string): Promise<void> {
    const notification = this.notifications.get(notificationId);
    if (notification) {
      notification.read = true;
    }
  }

  /**
   * Mark all notifications as read for a user
   */
  async markAllAsRead(userId: string): Promise<void> {
    for (const notification of this.notifications.values()) {
      if (notification.userId === userId && !notification.read) {
        notification.read = true;
      }
    }
  }

  /**
   * Dismiss a notification
   */
  async dismissNotification(notificationId: string): Promise<void> {
    const notification = this.notifications.get(notificationId);
    if (notification) {
      notification.dismissed = true;
    }
  }

  /**
   * Get unread notification count
   */
  async getUnreadCount(userId: string): Promise<number> {
    return Array.from(this.notifications.values()).filter(
      n => (n.userId === userId || n.walletAddress === userId) && !n.read && !n.dismissed
    ).length;
  }

  /**
   * Delete old notifications
   */
  async cleanupOldNotifications(maxAgeMs: number = 30 * 24 * 60 * 60 * 1000): Promise<number> {
    const cutoff = Date.now() - maxAgeMs;
    let deleted = 0;

    for (const [id, notification] of this.notifications.entries()) {
      if (notification.createdAt < cutoff) {
        this.notifications.delete(id);
        deleted++;
      }
    }

    return deleted;
  }

  // ============================================================================
  // Price Alerts
  // ============================================================================

  /**
   * Create a price alert
   */
  async createPriceAlert(
    userId: string,
    tokenAddress: string,
    tokenSymbol: string,
    targetPrice: number,
    condition: 'above' | 'below' | 'crosses'
  ): Promise<PriceAlert> {
    const id = this.generateId();
    
    const alert: PriceAlert = {
      id,
      tokenAddress,
      tokenSymbol,
      targetPrice,
      condition,
      triggered: false,
      createdAt: Date.now(),
      userId,
    };

    this.priceAlerts.set(id, alert);
    return alert;
  }

  /**
   * Get user's price alerts
   */
  async getPriceAlerts(userId: string): Promise<PriceAlert[]> {
    return Array.from(this.priceAlerts.values())
      .filter(a => a.userId === userId && !a.triggered);
  }

  /**
   * Delete a price alert
   */
  async deletePriceAlert(alertId: string, userId: string): Promise<boolean> {
    const alert = this.priceAlerts.get(alertId);
    
    if (!alert || alert.userId !== userId) {
      return false;
    }

    return this.priceAlerts.delete(alertId);
  }

  /**
   * Start monitoring prices for alerts
   */
  private startPriceMonitoring(): void {
    this.priceCheckInterval = setInterval(async () => {
      await this.checkPriceAlerts();
    }, 60000); // Check every minute
  }

  /**
   * Check and trigger price alerts
   */
  private async checkPriceAlerts(): Promise<void> {
    // This would connect to a price oracle in production
    const mockPrices: Record<string, number> = {
      'ETH': 2450,
      'BTC': 62500,
      'LINK': 18.5,
      'UNI': 12.5,
    };

    for (const alert of this.priceAlerts.values()) {
      if (alert.triggered) continue;

      const currentPrice = mockPrices[alert.tokenSymbol];
      if (currentPrice === undefined) continue;

      let shouldTrigger = false;

      switch (alert.condition) {
        case 'above':
          shouldTrigger = currentPrice >= alert.targetPrice;
          break;
        case 'below':
          shouldTrigger = currentPrice <= alert.targetPrice;
          break;
        case 'crosses':
          // Would need previous price to detect crosses
          shouldTrigger = Math.abs(currentPrice - alert.targetPrice) < currentPrice * 0.001;
          break;
      }

      if (shouldTrigger) {
        alert.triggered = true;
        alert.triggeredAt = Date.now();

        await this.sendNotification(alert.userId, {
          type: 'price_alert',
          title: `Price Alert: ${alert.tokenSymbol}`,
          message: `${alert.tokenSymbol} is now ${alert.condition} $${alert.targetPrice}. Current price: $${currentPrice}`,
          data: {
            tokenSymbol: alert.tokenSymbol,
            targetPrice: alert.targetPrice,
            currentPrice,
            condition: alert.condition,
          },
          channels: ['in_app', 'push'],
          userId: alert.userId,
        });
      }
    }
  }

  /**
   * Stop price monitoring
   */
  stopPriceMonitoring(): void {
    if (this.priceCheckInterval) {
      clearInterval(this.priceCheckInterval);
      this.priceCheckInterval = null;
    }
  }

  // ============================================================================
  // User Preferences
  // ============================================================================

  /**
   * Get user notification preferences
   */
  getPreferences(userId: string): NotificationPreferences {
    return this.preferences.get(userId) || { ...DEFAULT_PREFERENCES };
  }

  /**
   * Update user notification preferences
   */
  async updatePreferences(
    userId: string,
    preferences: Partial<NotificationPreferences>
  ): Promise<NotificationPreferences> {
    const current = this.getPreferences(userId);
    const updated = { ...current, ...preferences };
    this.preferences.set(userId, updated);
    return updated;
  }

  /**
   * Check if a channel is enabled for a notification type
   */
  private isChannelEnabled(
    prefs: NotificationPreferences,
    channel: NotificationChannel,
    type: NotificationType
  ): boolean {
    switch (channel) {
      case 'email':
        if (!prefs.email) return false;
        break;
      case 'push':
        if (!prefs.push) return false;
        break;
      case 'sms':
        if (!prefs.sms) return false;
        break;
      case 'discord':
        if (!prefs.discord) return false;
        break;
      case 'telegram':
        if (!prefs.telegram) return false;
        break;
    }

    // Check type-specific settings
    switch (type) {
      case 'price_alert':
        return prefs.priceAlerts;
      case 'order_filled':
      case 'order_partial':
      case 'order_cancelled':
        return prefs.orderUpdates;
      case 'swap_success':
      case 'swap_failed':
        return prefs.swapUpdates;
      case 'liquidity_added':
      case 'liquidity_removed':
        return prefs.liquidityUpdates;
      case 'large_transaction':
        return prefs.largeTransactions;
      case 'system_update':
      case 'security_alert':
        return prefs.systemUpdates;
    }

    return true;
  }

  // ============================================================================
  // Channel Sending
  // ============================================================================

  /**
   * Send notification to a specific channel
   */
  private async sendToChannel(
    userId: string,
    notification: Notification,
    channel: NotificationChannel
  ): Promise<void> {
    // In production, this would integrate with actual services
    switch (channel) {
      case 'email':
        await this.sendEmail(userId, notification);
        break;
      case 'push':
        await this.sendPush(userId, notification);
        break;
      case 'sms':
        await this.sendSMS(userId, notification);
        break;
      case 'discord':
        await this.sendDiscord(userId, notification);
        break;
      case 'telegram':
        await this.sendTelegram(userId, notification);
        break;
      case 'in_app':
      default:
        // Already stored in memory
        break;
    }
  }

  private async sendEmail(userId: string, notification: Notification): Promise<void> {
    // In production, integrate with email service (SendGrid, etc.)
    console.log(`[EMAIL] To: ${userId}, Subject: ${notification.title}, Body: ${notification.message}`);
  }

  private async sendPush(userId: string, notification: Notification): Promise<void> {
    // In production, integrate with push notification service (Firebase, etc.)
    console.log(`[PUSH] User: ${userId}, Title: ${notification.title}, Body: ${notification.message}`);
  }

  private async sendSMS(userId: string, notification: Notification): Promise<void> {
    // In production, integrate with SMS service (Twilio, etc.)
    console.log(`[SMS] User: ${userId}, Message: ${notification.title}: ${notification.message}`);
  }

  private async sendDiscord(userId: string, notification: Notification): Promise<void> {
    // In production, use Discord webhook
    console.log(`[DISCORD] User: ${userId}, Title: ${notification.title}, Body: ${notification.message}`);
  }

  private async sendTelegram(userId: string, notification: Notification): Promise<void> {
    // In production, use Telegram Bot API
    console.log(`[TELEGRAM] User: ${userId}, Title: ${notification.title}, Body: ${notification.message}`);
  }

  // ============================================================================
  // Subscription System
  // ============================================================================

  /**
   * Subscribe to notifications
   */
  subscribe(
    userId: string,
    callback: (notification: Notification) => void
  ): () => void {
    if (!this.subscribers.has(userId)) {
      this.subscribers.set(userId, []);
    }
    
    this.subscribers.get(userId)!.push(callback);

    // Return unsubscribe function
    return () => {
      const userSubs = this.subscribers.get(userId);
      if (userSubs) {
        const index = userSubs.indexOf(callback);
        if (index > -1) {
          userSubs.splice(index, 1);
        }
      }
    };
  }

  /**
   * Notify all subscribers of a user
   */
  private notifySubscribers(userId: string, notification: Notification): void {
    const userSubs = this.subscribers.get(userId);
    if (userSubs) {
      for (const callback of userSubs) {
        try {
          callback(notification);
        } catch (error) {
          console.error('Error in notification subscriber:', error);
        }
      }
    }
  }

  // ============================================================================
  // Helper Methods
  // ============================================================================

  private generateId(): string {
    return 'notif_' + Date.now() + '_' + Math.random().toString(36).slice(2, 10);
  }

  /**
   * Create notification from template
   */
  async sendTemplatedNotification(
    userId: string,
    templateId: string,
    variables: Record<string, string>,
    channels?: NotificationChannel[]
  ): Promise<Notification | null> {
    const template = this.getTemplate(templateId);
    if (!template) return null;

    const title = this.interpolate(template.subject, variables);
    const message = this.interpolate(template.bodyTemplate, variables);

    return this.sendNotification(userId, {
      type: template.type,
      title,
      message,
      channels: channels || template.channels,
    });
  }

  private getTemplate(templateId: string): NotificationTemplate | null {
    const templates: Record<string, NotificationTemplate> = {
      'swap_success': {
        id: 'swap_success',
        type: 'swap_success',
        subject: 'Swap Completed Successfully',
        bodyTemplate: 'Your swap of {inputAmount} {inputToken} for {outputAmount} {outputToken} has been completed.',
        channels: ['in_app', 'push'],
      },
      'price_alert': {
        id: 'price_alert',
        type: 'price_alert',
        subject: 'Price Alert: {tokenSymbol}',
        bodyTemplate: '{tokenSymbol} has reached {targetPrice}. Current price: {currentPrice}',
        channels: ['in_app', 'push', 'email'],
      },
    };

    return templates[templateId] || null;
  }

  private interpolate(text: string, variables: Record<string, string>): string {
    return text.replace(/\{(\w+)\}/g, (_, key) => variables[key] || `{${key}}`);
  }
}

// ============================================================================
// Singleton Export
// ============================================================================

export const notificationService = new NotificationService();
export default NotificationService;