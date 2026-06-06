//! Intent Matching
//! 
//! Matches intents with solutions.

use std::collections::HashMap;
use std::sync::{Arc, RwLock};

#[derive(Debug, Clone)]
pub struct Match {
    pub intent_id: String,
    pub solution_id: String,
    pub score: f64,
}

pub struct IntentMatcher {
    matches: RwLock<HashMap<String, Match>>,
}

impl IntentMatcher {
    pub fn new() -> Self {
        Self {
            matches: RwLock::new(HashMap::new()),
        }
    }
    
    pub fn match_intent(&self, intent_id: &str, solution_id: &str, score: f64) {
        let match_ = Match {
            intent_id: intent_id.to_string(),
            solution_id: solution_id.to_string(),
            score,
        };
        
        self.matches.write().unwrap().insert(intent_id.to_string(), match_);
    }
    
    pub fn get_match(&self, intent_id: &str) -> Option<Match> {
        self.matches.read().unwrap().get(intent_id).cloned()
    }
}

impl Default for IntentMatcher {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_matcher() {
        let matcher = IntentMatcher::new();
        matcher.match_intent("intent-1", "solution-1", 0.95);
        assert!(matcher.get_match("intent-1").is_some());
    }
}