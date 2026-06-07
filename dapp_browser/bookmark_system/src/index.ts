/**
 * TigerSwap DApp Browser - Bookmark System
 * 
 * Native DApp bookmark management.
 * Zero external dependencies - fully native implementation.
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

export interface Bookmark {
  id: string;
  url: string;
  title: string;
  description?: string;
  favicon?: string;
  category: string;
  tags: string[];
  rating: number;
  lastVisited?: number;
  visitCount: number;
  isVerified: boolean;
  createdAt: number;
}

// Default categories
const DEFAULT_CATEGORIES = [
  'DeFi',
  'NFT',
  'Games',
  'Tools',
  'Social',
  'Finance',
  'Education',
  'Other',
];

export class BookmarkSystem {
  private bookmarks: Map<string, Bookmark>;
  private categories: Set<string>;

  constructor() {
    this.bookmarks = new Map();
    this.categories = new Set(DEFAULT_CATEGORIES);
    this.loadBookmarks();
  }

  /**
   * Load bookmarks from storage
   */
  private async loadBookmarks(): Promise<void> {
    // Would load from localStorage in browser
    // Placeholder for native implementation
  }

  /**
   * Add bookmark
   */
  addBookmark(bookmark: Omit<Bookmark, 'id' | 'createdAt' | 'visitCount' | 'rating'>): string {
    const id = this.generateId();
    
    const newBookmark: Bookmark = {
      ...bookmark,
      id,
      createdAt: Date.now(),
      visitCount: 0,
      rating: 0,
    };

    this.bookmarks.set(id, newBookmark);
    return id;
  }

  /**
   * Remove bookmark
   */
  removeBookmark(id: string): void {
    this.bookmarks.delete(id);
  }

  /**
   * Update bookmark
   */
  updateBookmark(id: string, updates: Partial<Bookmark>): void {
    const bookmark = this.bookmarks.get(id);
    if (bookmark) {
      this.bookmarks.set(id, { ...bookmark, ...updates });
    }
  }

  /**
   * Get bookmark
   */
  getBookmark(id: string): Bookmark | undefined {
    return this.bookmarks.get(id);
  }

  /**
   * Get all bookmarks
   */
  getAllBookmarks(): Bookmark[] {
    return Array.from(this.bookmarks.values());
  }

  /**
   * Get bookmarks by category
   */
  getByCategory(category: string): Bookmark[] {
    return this.getAllBookmarks().filter(b => b.category === category);
  }

  /**
   * Get bookmarks by tag
   */
  getByTag(tag: string): Bookmark[] {
    return this.getAllBookmarks().filter(b => b.tags.includes(tag));
  }

  /**
   * Search bookmarks
   */
  search(query: string): Bookmark[] {
    const q = query.toLowerCase();
    return this.getAllBookmarks().filter(b => 
      b.title.toLowerCase().includes(q) ||
      b.description?.toLowerCase().includes(q) ||
      b.url.toLowerCase().includes(q)
    );
  }

  /**
   * Visit bookmark
   */
  visit(id: string): void {
    const bookmark = this.bookmarks.get(id);
    if (bookmark) {
      bookmark.visitCount++;
      bookmark.lastVisited = Date.now();
    }
  }

  /**
   * Rate bookmark
   */
  rate(id: string, rating: number): void {
    const bookmark = this.bookmarks.get(id);
    if (bookmark) {
      const oldCount = bookmark.rating * bookmark.visitCount;
      bookmark.rating = (oldCount + rating) / (bookmark.visitCount + 1);
    }
  }

  /**
   * Get top bookmarks
   */
  getTopBookmarks(limit: number = 10): Bookmark[] {
    return this.getAllBookmarks()
      .sort((a, b) => b.rating - a.rating)
      .slice(0, limit);
  }

  /**
   * Get recently visited
   */
  getRecentlyVisited(limit: number = 10): Bookmark[] {
    return this.getAllBookmarks()
      .filter(b => b.lastVisited)
      .sort((a, b) => (b.lastVisited || 0) - (a.lastVisited || 0))
      .slice(0, limit);
  }

  /**
   * Get popular bookmarks
   */
  getPopular(limit: number = 10): Bookmark[] {
    return this.getAllBookmarks()
      .sort((a, b) => b.visitCount - a.visitCount)
      .slice(0, limit);
  }

  /**
   * Add category
   */
  addCategory(category: string): void {
    this.categories.add(category);
  }

  /**
   * Get categories
   */
  getCategories(): string[] {
    return Array.from(this.categories);
  }

  /**
   * Get bookmark count
   */
  getCount(): number {
    return this.bookmarks.size;
  }

  /**
   * Export bookmarks
   */
  exportBookmarks(): string {
    return JSON.stringify(this.getAllBookmarks(), null, 2);
  }

  /**
   * Import bookmarks
   */
  importBookmarks(data: string): void {
    const imported = JSON.parse(data) as Bookmark[];
    for (const bookmark of imported) {
      this.bookmarks.set(bookmark.id, bookmark);
    }
  }

  private generateId(): string {
    return 'bm_' + Math.random().toString(36).substr(2, 9);
  }
}

export default BookmarkSystem;