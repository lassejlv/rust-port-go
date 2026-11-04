#[derive(Copy, Clone)]
pub struct Sample {
    pub input: i64,
    pub result: i64,
}

pub fn sample(input: i64) -> Sample {
    Sample { input, result: crate::math::fib(input) }
}
