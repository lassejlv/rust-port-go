// Development-only oracle: compile the original pinned Cargo queue unchanged.
// This program is never used by the production Go queue.
// MIT OR Apache-2.0, as the included upstream source.
use std::sync::{mpsc, Arc};
use std::thread;
use std::time::Duration;

mod upstream {
    include!(env!("CARGO_QUEUE_SOURCE"));
}

fn main() {
    let q = upstream::Queue::new(2);
    println!("empty={:?}", q.pop(Duration::ZERO));
    q.push(10);
    q.push_bounded(20);
    q.push(30);
    println!("fifo={:?}", q.pop(Duration::ZERO));
    println!("drain={:?}", q.try_pop_all());
    println!("empty-drain={:?}", q.try_pop_all());
    let q = Arc::new(upstream::Queue::new(1));
    let other = q.clone();
    let (started, start) = mpsc::channel();
    let huge = thread::spawn(move || {
        started.send(()).unwrap();
        other.pop(Duration::new(u64::MAX, 999_999_999))
    });
    start.recv().unwrap();
    thread::sleep(Duration::from_millis(20));
    q.push(99);
    println!("huge={:?}", huge.join().unwrap());
    let q = Arc::new(upstream::Queue::new(1));
    q.push(1);
    let other = q.clone();
    let (done, result) = mpsc::channel();
    let producer = thread::spawn(move || { other.push_bounded(2); done.send(()).unwrap(); });
    println!("bounded-blocked={}", result.recv_timeout(Duration::from_millis(20)).is_err());
    println!("bounded-first={:?}", q.pop(Duration::ZERO));
    result.recv_timeout(Duration::from_secs(2)).unwrap();
    producer.join().unwrap();
    println!("bounded-second={:?}", q.pop(Duration::ZERO));

    let q = Arc::new(upstream::Queue::new(0));
    let other = q.clone();
    let (done, result) = mpsc::channel();
    thread::spawn(move || { other.push_bounded(101); let _ = done.send(()); });
    q.push(7);
    println!("zero-pop={:?}", q.pop(Duration::ZERO));
    println!("zero-drain={:?}", q.try_pop_all());
    println!("zero-blocked={}", result.recv_timeout(Duration::from_millis(20)).is_err());

    let d = Duration::new(1, u32::MAX);
    println!("duration-normalized={},{}", d.as_secs(), d.subsec_nanos());
    let d = Duration::new(u64::MAX, 999_999_999);
    println!("duration-max={},{}", d.as_secs(), d.subsec_nanos());

    for (label, bound) in [("0", 0), ("1", 1), ("2", 2), ("17", 17), ("usize::MAX", usize::MAX)] {
        println!("trace-bound={label}");
        let q = upstream::Queue::new(bound);
        let mut count = 0;
        let mut seed = 0x7310_acbfu64;
        for step in 0..20_000 {
            seed = seed.wrapping_mul(6364136223846793005).wrapping_add(1442695040888963407);
            match seed >> 61 {
                0..=4 => {
                    if seed & 1 == 0 && count < bound { q.push_bounded(step); } else { q.push(step); }
                    count += 1;
                }
                5..=6 => {
                    println!("P{step}:{:?}", q.pop(Duration::ZERO));
                    count = count.saturating_sub(1);
                }
                7 => { println!("D{step}:{:?}", q.try_pop_all()); count = 0; }
                _ => unreachable!(),
            }
        }
        println!("trace-final={:?}", q.try_pop_all());
    }
}
